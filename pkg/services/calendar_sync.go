package services

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const createCalendarSyncOperation = "create_calendar_sync"

// ErrSourceOwnedMarker indicates that provider synchronization owns one event.
var ErrSourceOwnedMarker = errors.New("source-owned marker fields cannot change locally")

// CalendarSyncService owns provider synchronization and source reconciliation.
type CalendarSyncService struct {
	database *gorm.DB
	adapter  CalendarProviderAdapter
	cipher   *CredentialCipher
	now      func() time.Time
}

func NewCalendarSyncService(database *gorm.DB, adapter CalendarProviderAdapter, credentialCipher *CredentialCipher, now func() time.Time) (*CalendarSyncService, error) {
	if database == nil || adapter == nil || credentialCipher == nil || now == nil {
		return nil, errors.New("calendar synchronization dependencies are required")
	}
	return &CalendarSyncService{database: database, adapter: adapter, cipher: credentialCipher, now: now}, nil
}

// Create synchronizes one organizer-owned source mapping and records its result.
func (service *CalendarSyncService) Create(ctx context.Context, organizerID string, mappingID string, idempotencyKey string) (*models.CalendarSync, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, ErrIdempotencyKeyRequired
	}
	payload, _ := json.Marshal(map[string]string{"mapping_id": mappingID})
	keyHash := sha256.Sum256([]byte(idempotencyKey))
	requestHash := sha256.Sum256(payload)
	if existing, found, lookupError := service.readIdempotentSync(ctx, organizerID, keyHash[:], requestHash[:]); lookupError != nil || found {
		return existing, found, lookupError
	}
	mapping, credential, mappingError := service.mappingCredential(ctx, organizerID, mappingID)
	if mappingError != nil {
		return nil, false, mappingError
	}
	startedAt := service.now().UTC()
	synchronization, syncError := models.NewCalendarSync(mapping.ID, startedAt)
	if syncError != nil {
		return nil, false, syncError
	}
	synchronization.State = models.CalendarSyncRunning
	if createError := service.database.WithContext(ctx).Create(synchronization).Error; createError != nil {
		return nil, false, fmt.Errorf("create calendar synchronization: %w", createError)
	}
	cursor := ""
	if mapping.SyncCursor != nil {
		cursor = *mapping.SyncCursor
	}
	batch, providerError := service.adapter.SynchronizeEvents(ctx, credential, mapping.ProviderCalendarID, cursor)
	completeReconciliation := cursor == ""
	if errors.Is(providerError, ErrCalendarSyncCursorRejected) && cursor != "" {
		batch, providerError = service.adapter.SynchronizeEvents(ctx, credential, mapping.ProviderCalendarID, "")
		completeReconciliation = true
	}
	if providerError != nil {
		service.failSync(ctx, synchronization, "provider_failed")
		return nil, false, providerError
	}
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lockedMapping models.SourceCalendarMapping
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedMapping, "id = ?", mapping.ID).Error; findError != nil {
			return findError
		}
		if applyError := service.applyBatch(transaction, &lockedMapping, synchronization.StartedAt, batch, completeReconciliation); applyError != nil {
			return applyError
		}
		finishedAt := service.now().UTC()
		synchronization.State = models.CalendarSyncSucceeded
		synchronization.FinishedAt = &finishedAt
		if saveError := transaction.Save(synchronization).Error; saveError != nil {
			return saveError
		}
		record, recordError := models.NewIdempotencyRecord(organizerID, createCalendarSyncOperation, keyHash[:], requestHash[:], http.StatusAccepted, "calendar_sync", synchronization.ID, finishedAt.Add(idempotencyLifetime))
		if recordError != nil {
			return recordError
		}
		return transaction.Create(record).Error
	})
	if transactionError != nil {
		service.failSync(ctx, synchronization, "persistence_failed")
		return nil, false, transactionError
	}
	return synchronization, false, nil
}

// Read returns one organizer-owned synchronization result.
func (service *CalendarSyncService) Read(ctx context.Context, organizerID string, syncID string) (*models.CalendarSync, error) {
	var synchronization models.CalendarSync
	if findError := service.database.WithContext(ctx).First(&synchronization, "id = ?", syncID).Error; findError != nil {
		return nil, findError
	}
	var ownerID string
	ownerError := service.database.WithContext(ctx).Table("source_calendar_mappings AS mappings").
		Select("connections.organizer_id").
		Joins("JOIN calendar_connections AS connections ON connections.id = mappings.connection_id AND connections.deleted_at IS NULL").
		Where("mappings.id = ? AND mappings.deleted_at IS NULL", synchronization.MappingID).Scan(&ownerID).Error
	if ownerError != nil {
		return nil, ownerError
	}
	if ownerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &synchronization, nil
}

func (service *CalendarSyncService) mappingCredential(ctx context.Context, organizerID string, mappingID string) (*models.SourceCalendarMapping, CalendarProviderCredential, error) {
	var mapping models.SourceCalendarMapping
	if findError := service.database.WithContext(ctx).Preload("Connection").First(&mapping, "id = ?", mappingID).Error; findError != nil {
		return nil, CalendarProviderCredential{}, findError
	}
	if mapping.Connection.OrganizerID != organizerID {
		return nil, CalendarProviderCredential{}, ErrResourceForbidden
	}
	credential, credentialError := currentCalendarCredential(ctx, service.database, service.adapter, service.cipher, &mapping.Connection, service.now())
	return &mapping, credential, credentialError
}

func (service *CalendarSyncService) applyBatch(database *gorm.DB, mapping *models.SourceCalendarMapping, startedAt time.Time, batch ProviderEventBatch, complete bool) error {
	seen := make(map[string]struct{}, len(batch.Events))
	for _, providerEvent := range batch.Events {
		seen[providerEvent.ID] = struct{}{}
		if providerEvent.Status == "cancelled" {
			if deleteError := deleteExternalEvent(database, mapping.ID, providerEvent.ID); deleteError != nil {
				return deleteError
			}
			continue
		}
		if applyError := upsertExternalEvent(database, mapping, startedAt, providerEvent); applyError != nil {
			return applyError
		}
	}
	if complete {
		var links []models.ExternalEventLink
		if findError := database.Where("mapping_id = ?", mapping.ID).Find(&links).Error; findError != nil {
			return findError
		}
		for _, link := range links {
			if _, found := seen[link.ProviderEventID]; found {
				continue
			}
			if deleteError := deleteExternalEvent(database, mapping.ID, link.ProviderEventID); deleteError != nil {
				return deleteError
			}
		}
	}
	if boundsError := recalculateMappingLaneBounds(database, mapping.ID); boundsError != nil {
		return boundsError
	}
	mapping.SyncCursor = &batch.NextSyncCursor
	if updateError := database.Model(mapping).Update("sync_cursor", batch.NextSyncCursor).Error; updateError != nil {
		return updateError
	}
	return NormalizeLaneOrder(database, mapping.CalendarID)
}

func recalculateMappingLaneBounds(database *gorm.DB, mappingID string) error {
	var events []models.Event
	if findError := database.Model(&models.Event{}).
		Joins("JOIN external_event_links ON external_event_links.event_id = events.id AND external_event_links.deleted_at IS NULL").
		Where("external_event_links.mapping_id = ?", mappingID).Find(&events).Error; findError != nil {
		return findError
	}
	lastByLane := make(map[string]time.Time)
	for eventIndex := range events {
		_, markerEnd, boundsError := events[eventIndex].MarkerBounds()
		if boundsError != nil {
			return boundsError
		}
		if markerEnd.After(lastByLane[events[eventIndex].LaneID]) {
			lastByLane[events[eventIndex].LaneID] = markerEnd
		}
	}
	for laneID, markerEnd := range lastByLane {
		var lane models.Lane
		if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
			return findError
		}
		if !markerEnd.After(lane.StartsAt) {
			markerEnd = lane.StartsAt.Add(time.Nanosecond)
		}
		if updateError := database.Model(&lane).Update("ends_at", markerEnd).Error; updateError != nil {
			return updateError
		}
		if boundsError := RecalculateTemporalLaneBounds(database, laneID); boundsError != nil {
			return boundsError
		}
	}
	return nil
}

func upsertExternalEvent(database *gorm.DB, mapping *models.SourceCalendarMapping, startedAt time.Time, providerEvent ProviderEvent) error {
	eventTime, boundsStart, boundsEnd, timeError := providerEventTime(providerEvent)
	if timeError != nil {
		return fmt.Errorf("convert source event time: %w", timeError)
	}
	var link models.ExternalEventLink
	findError := database.First(&link, "mapping_id = ? AND provider_event_id = ?", mapping.ID, providerEvent.ID).Error
	if findError == nil {
		var event models.Event
		if eventError := database.First(&event, "id = ?", link.EventID).Error; eventError != nil {
			return eventError
		}
		candidate, candidateError := models.NewEvent(event.LaneID, providerEvent.Title, providerEvent.Description, nil, eventRelation(event), eventTime)
		if candidateError != nil {
			return fmt.Errorf("construct updated source event: %w", candidateError)
		}
		if boundsError := expandSourceLane(database, event.LaneID, startedAt, boundsStart, boundsEnd); boundsError != nil {
			return boundsError
		}
		if updateError := database.Model(&event).Updates(map[string]any{"title": candidate.Title, "description": candidate.Description, "time_shape": candidate.TimeShape, "at": candidate.At, "starts_at": candidate.StartsAt, "ends_at": candidate.EndsAt, "start_date": candidate.StartDate, "end_date": candidate.EndDate, "timezone": candidate.Timezone}).Error; updateError != nil {
			return updateError
		}
		ownerID, ownerError := event.OwnerID(database)
		if ownerError != nil {
			return ownerError
		}
		return RecalculateDerivedMarkersForAnchor(database, ownerID, models.DerivedAnchorEvent, event.ID)
	}
	if !errors.Is(findError, gorm.ErrRecordNotFound) {
		return findError
	}
	lane, series, laneError := sourceLane(database, mapping, startedAt, boundsStart, boundsEnd, providerEvent)
	if laneError != nil {
		return fmt.Errorf("prepare source event lane: %w", laneError)
	}
	relation := models.IndependentEventRelation()
	if series != nil {
		relation, laneError = models.SeriesOccurrenceRelation(series.ID)
		if laneError != nil {
			return laneError
		}
	}
	event, eventError := models.NewEvent(lane.ID, providerEvent.Title, providerEvent.Description, nil, relation, eventTime)
	if eventError != nil {
		return fmt.Errorf("construct source event: %w", eventError)
	}
	if createError := database.Create(event).Error; createError != nil {
		return createError
	}
	var providerSeriesID *string
	if providerEvent.SeriesID != "" {
		providerSeriesID = &providerEvent.SeriesID
	}
	externalLink, externalError := models.NewExternalEventLink(mapping.ID, event.ID, providerEvent.ID, providerSeriesID)
	if externalError != nil {
		return externalError
	}
	return database.Create(externalLink).Error
}

func sourceLane(database *gorm.DB, mapping *models.SourceCalendarMapping, startedAt time.Time, markerStart time.Time, markerEnd time.Time, providerEvent ProviderEvent) (*models.Lane, *models.EventSeries, error) {
	if providerEvent.SeriesID != "" {
		var externalSeries models.ExternalEventSeriesLink
		findError := database.First(&externalSeries, "mapping_id = ? AND provider_series_id = ?", mapping.ID, providerEvent.SeriesID).Error
		if findError == nil {
			var series models.EventSeries
			if seriesError := database.First(&series, "id = ?", externalSeries.EventSeriesID).Error; seriesError != nil {
				return nil, nil, seriesError
			}
			if boundsError := expandSourceLane(database, series.LaneID, startedAt, markerStart, markerEnd); boundsError != nil {
				return nil, nil, boundsError
			}
			var lane models.Lane
			if laneError := database.First(&lane, "id = ?", series.LaneID).Error; laneError != nil {
				return nil, nil, laneError
			}
			return &lane, &series, nil
		}
		if !errors.Is(findError, gorm.ErrRecordNotFound) {
			return nil, nil, findError
		}
	}
	start := startedAt
	if markerStart.Before(start) {
		start = markerStart
	}
	end := markerEnd
	if !end.After(start) {
		end = start.Add(time.Nanosecond)
	}
	order, orderError := models.NextLaneDisplayOrder(database, mapping.CalendarID)
	if orderError != nil {
		return nil, nil, orderError
	}
	lane, laneError := models.NewFiniteLane(mapping.CalendarID, providerEvent.Title, start, end, order)
	if laneError != nil {
		return nil, nil, laneError
	}
	if createError := database.Create(lane).Error; createError != nil {
		return nil, nil, createError
	}
	if providerEvent.SeriesID == "" {
		return lane, nil, nil
	}
	timezone, timezoneError := models.NewTimezone(providerEvent.Timezone)
	if timezoneError != nil {
		return nil, nil, timezoneError
	}
	series, seriesError := models.NewEventSeries(lane.ID, timezone, models.EventSourceGoogle, nil)
	if seriesError != nil {
		return nil, nil, seriesError
	}
	if createError := database.Create(series).Error; createError != nil {
		return nil, nil, createError
	}
	externalSeries, externalError := models.NewExternalEventSeriesLink(mapping.ID, series.ID, providerEvent.SeriesID)
	if externalError != nil {
		return nil, nil, externalError
	}
	if createError := database.Create(externalSeries).Error; createError != nil {
		return nil, nil, createError
	}
	return lane, series, nil
}

func expandSourceLane(database *gorm.DB, laneID string, trackedAt time.Time, markerStart time.Time, markerEnd time.Time) error {
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
		return findError
	}
	start := lane.StartsAt
	if trackedAt.Before(start) {
		start = trackedAt
	}
	if markerStart.Before(start) {
		start = markerStart
	}
	end := markerEnd
	if lane.EndsAt != nil && lane.EndsAt.After(end) {
		end = *lane.EndsAt
	}
	if !end.After(start) {
		end = start.Add(time.Nanosecond)
	}
	return database.Model(&lane).Updates(map[string]any{"starts_at": start, "ends_at": end}).Error
}

func providerEventTime(event ProviderEvent) (models.EventTime, time.Time, time.Time, error) {
	timezone, timezoneError := models.NewTimezone(event.Timezone)
	if timezoneError != nil {
		return models.EventTime{}, time.Time{}, time.Time{}, timezoneError
	}
	if event.StartDate != "" || event.EndDate != "" {
		startDate, startError := models.NewLocalDate(event.StartDate)
		endDate, endError := models.NewLocalDate(event.EndDate)
		if startError != nil {
			return models.EventTime{}, time.Time{}, time.Time{}, startError
		}
		if endError != nil {
			return models.EventTime{}, time.Time{}, time.Time{}, endError
		}
		eventTime, eventTimeError := models.NewAllDayEventTime(startDate, endDate, timezone)
		if eventTimeError != nil {
			return models.EventTime{}, time.Time{}, time.Time{}, eventTimeError
		}
		temporary, _ := models.NewEvent("temporary", event.Title, event.Description, nil, models.IndependentEventRelation(), eventTime)
		start, end, boundsError := temporary.MarkerBounds()
		return eventTime, start, end, boundsError
	}
	if event.At != nil {
		eventTime, eventTimeError := models.NewPointEventTime(*event.At, timezone)
		return eventTime, event.At.UTC(), event.At.UTC(), eventTimeError
	}
	if event.StartsAt == nil || event.EndsAt == nil {
		return models.EventTime{}, time.Time{}, time.Time{}, models.ErrEventTimeInvalid
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(*event.StartsAt, *event.EndsAt, timezone)
	return eventTime, event.StartsAt.UTC(), event.EndsAt.UTC(), eventTimeError
}

func eventRelation(event models.Event) models.EventRelation {
	if event.EventSeriesID != nil {
		relation, _ := models.SeriesOccurrenceRelation(*event.EventSeriesID)
		return relation
	}
	if event.AnchorEventID != nil {
		relation, _ := models.DependentEventRelation(*event.AnchorEventID)
		return relation
	}
	return models.IndependentEventRelation()
}

func deleteExternalEvent(database *gorm.DB, mappingID string, providerEventID string) error {
	var link models.ExternalEventLink
	findError := database.First(&link, "mapping_id = ? AND provider_event_id = ?", mappingID, providerEventID).Error
	if errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil
	}
	if findError != nil {
		return findError
	}
	var event models.Event
	if eventError := database.First(&event, "id = ?", link.EventID).Error; eventError != nil {
		return eventError
	}
	if useError := requireSourceEventUnused(database, event.ID); useError != nil {
		return useError
	}
	laneID := event.LaneID
	if derivedDeleteError := DeleteDerivedMarkersForAnchor(database, models.DerivedAnchorEvent, event.ID); derivedDeleteError != nil {
		return derivedDeleteError
	}
	if deleteError := database.Unscoped().Delete(&link).Error; deleteError != nil {
		return deleteError
	}
	if deleteError := database.Unscoped().Delete(&event).Error; deleteError != nil {
		return deleteError
	}
	var remaining int64
	if countError := database.Model(&models.Event{}).Where("lane_id = ?", laneID).Count(&remaining).Error; countError != nil {
		return countError
	}
	if remaining != 0 {
		return models.RecalculateFiniteLaneEnd(database, laneID)
	}
	if event.EventSeriesID != nil {
		if deleteError := database.Unscoped().Where("event_series_id = ?", *event.EventSeriesID).Delete(&models.ExternalEventSeriesLink{}).Error; deleteError != nil {
			return deleteError
		}
		if deleteError := database.Unscoped().Delete(&models.EventSeries{}, "id = ?", *event.EventSeriesID).Error; deleteError != nil {
			return deleteError
		}
	}
	return database.Unscoped().Delete(&models.Lane{}, "id = ?", laneID).Error
}

func (service *CalendarSyncService) failSync(ctx context.Context, synchronization *models.CalendarSync, code string) {
	finishedAt := service.now().UTC()
	synchronization.State, synchronization.FinishedAt, synchronization.ErrorCode = models.CalendarSyncFailed, &finishedAt, &code
	_ = service.database.WithContext(ctx).Save(synchronization).Error
}

func (service *CalendarSyncService) readIdempotentSync(ctx context.Context, organizerID string, keyHash []byte, requestHash []byte) (*models.CalendarSync, bool, error) {
	var record models.IdempotencyRecord
	findError := service.database.WithContext(ctx).First(&record, "organizer_id = ? AND operation = ? AND key_hash = ?", organizerID, createCalendarSyncOperation, keyHash).Error
	if errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if findError != nil {
		return nil, false, findError
	}
	if !record.ExpiresAt.After(service.now().UTC()) {
		if deleteError := service.database.WithContext(ctx).Unscoped().Delete(&record).Error; deleteError != nil {
			return nil, false, deleteError
		}
		return nil, false, nil
	}
	if subtle.ConstantTimeCompare(record.RequestHash, requestHash) != 1 {
		return nil, false, ErrIdempotencyConflict
	}
	var synchronization models.CalendarSync
	if findError := service.database.WithContext(ctx).First(&synchronization, "id = ?", record.ResourceID).Error; findError != nil {
		return nil, false, findError
	}
	return &synchronization, true, nil
}
