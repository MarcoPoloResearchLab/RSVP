package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrSourceOwnedMarker indicates that provider synchronization owns one event.
var ErrSourceOwnedMarker = errors.New("source-owned marker fields cannot change locally")

// CalendarSyncService owns provider-calendar synchronization and source reconciliation.
type CalendarSyncService struct {
	database *gorm.DB
	adapter  CalendarProviderAdapter
	cipher   *CredentialCipher
	now      func() time.Time
	mutex    sync.Mutex
}

func NewCalendarSyncService(database *gorm.DB, adapter CalendarProviderAdapter, credentialCipher *CredentialCipher, now func() time.Time) (*CalendarSyncService, error) {
	if database == nil || adapter == nil || credentialCipher == nil || now == nil {
		return nil, errors.New("calendar synchronization dependencies are required")
	}
	return &CalendarSyncService{database: database, adapter: adapter, cipher: credentialCipher, now: now}, nil
}

// Synchronize reconciles one organizer-owned provider calendar and records its result.
func (service *CalendarSyncService) Synchronize(ctx context.Context, organizerID string, syncStateID string) (*models.CalendarSync, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()

	syncState, credential, stateError := service.syncStateCredential(ctx, organizerID, syncStateID)
	if stateError != nil {
		return nil, stateError
	}
	startedAt := service.now().UTC()
	synchronization, syncError := models.NewCalendarSync(syncState.ID, startedAt)
	if syncError != nil {
		return nil, syncError
	}
	synchronization.State = models.CalendarSyncRunning
	if createError := service.database.WithContext(ctx).Create(synchronization).Error; createError != nil {
		return nil, fmt.Errorf("create calendar synchronization: %w", createError)
	}
	cursor := ""
	if syncState.SyncCursor != nil {
		cursor = *syncState.SyncCursor
	}
	batch, providerError := service.adapter.SynchronizeEvents(ctx, credential, syncState.ProviderCalendarID, cursor)
	completeReconciliation := cursor == ""
	if errors.Is(providerError, ErrCalendarSyncCursorRejected) && cursor != "" {
		batch, providerError = service.adapter.SynchronizeEvents(ctx, credential, syncState.ProviderCalendarID, "")
		completeReconciliation = true
	}
	if providerError != nil {
		return nil, errors.Join(providerError, service.failSync(ctx, synchronization, "provider_failed"))
	}
	if batch.NextSyncCursor == "" {
		return nil, errors.Join(errors.New("provider event batch has no sync cursor"), service.failSync(ctx, synchronization, "provider_failed"))
	}
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lockedState models.ProviderCalendarSyncState
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Mappings.Calendar").First(&lockedState, "id = ?", syncState.ID).Error; findError != nil {
			return findError
		}
		if applyError := service.applyBatch(transaction, &lockedState, synchronization.StartedAt, batch, completeReconciliation); applyError != nil {
			return applyError
		}
		finishedAt := service.now().UTC()
		synchronization.State = models.CalendarSyncSucceeded
		synchronization.FinishedAt = &finishedAt
		return transaction.Save(synchronization).Error
	})
	if transactionError != nil {
		return nil, errors.Join(transactionError, service.failSync(ctx, synchronization, "persistence_failed"))
	}
	return synchronization, nil
}

// SynchronizeSyncStates reconciles each provider calendar and continues after failures.
func (service *CalendarSyncService) SynchronizeSyncStates(ctx context.Context, organizerID string, syncStates []models.ProviderCalendarSyncState) error {
	var synchronizationErrors []error
	for stateIndex := range syncStates {
		if _, synchronizationError := service.Synchronize(ctx, organizerID, syncStates[stateIndex].ID); synchronizationError != nil {
			synchronizationErrors = append(synchronizationErrors, fmt.Errorf("synchronize provider calendar %s: %w", syncStates[stateIndex].ID, synchronizationError))
		}
	}
	return errors.Join(synchronizationErrors...)
}

func (service *CalendarSyncService) syncStateCredential(ctx context.Context, organizerID string, syncStateID string) (*models.ProviderCalendarSyncState, CalendarProviderCredential, error) {
	var syncState models.ProviderCalendarSyncState
	if findError := service.database.WithContext(ctx).Preload("Connection").First(&syncState, "id = ?", syncStateID).Error; findError != nil {
		return nil, CalendarProviderCredential{}, findError
	}
	if syncState.Connection.OrganizerID != organizerID {
		return nil, CalendarProviderCredential{}, ErrResourceForbidden
	}
	credential, credentialError := currentCalendarCredential(ctx, service.database, service.adapter, service.cipher, &syncState.Connection, service.now())
	return &syncState, credential, credentialError
}

func (service *CalendarSyncService) applyBatch(database *gorm.DB, syncState *models.ProviderCalendarSyncState, startedAt time.Time, batch ProviderEventBatch, complete bool) error {
	mappings := make(map[SemanticCalendarGroup]*models.SourceCalendarMapping, len(syncState.Mappings))
	for mappingIndex := range syncState.Mappings {
		mapping := &syncState.Mappings[mappingIndex]
		group := SemanticCalendarGroup(mapping.SemanticGroup)
		if group != SemanticCalendarGroupCalendar && group != SemanticCalendarGroupBirthdays {
			return errors.New("source calendar mapping has an unknown semantic group")
		}
		mappings[group] = mapping
	}
	placements, seriesPlacements, groupingError := providerSeriesPlacements(database, syncState.ID, batch.Changes, complete)
	if groupingError != nil {
		return groupingError
	}
	seen := make(map[string]struct{}, len(batch.Changes))
	for _, change := range batch.Changes {
		if change.ProviderEventID == "" {
			return errors.New("provider event change has no source identity")
		}
		seen[change.ProviderEventID] = struct{}{}
		if change.Deleted {
			if deleteError := deleteExternalEvent(database, syncState.ID, change.ProviderEventID); deleteError != nil {
				return deleteError
			}
			continue
		}
		placementGroup, found := placements[change.ProviderEventID]
		if !found {
			return fmt.Errorf("provider event %s has no placement group", change.ProviderEventID)
		}
		mapping, found := mappings[placementGroup]
		if !found {
			return fmt.Errorf("provider calendar %s has no %s mapping", syncState.ProviderCalendarID, placementGroup)
		}
		if applyError := upsertExternalEvent(database, syncState.ID, mapping, startedAt, change); applyError != nil {
			return applyError
		}
	}
	if complete {
		var links []models.ExternalEventLink
		if findError := database.Where("sync_state_id = ?", syncState.ID).Find(&links).Error; findError != nil {
			return findError
		}
		for _, link := range links {
			if _, found := seen[link.ProviderEventID]; found {
				continue
			}
			if deleteError := deleteExternalEvent(database, syncState.ID, link.ProviderEventID); deleteError != nil {
				return deleteError
			}
		}
	}
	for providerSeriesID, placementGroup := range seriesPlacements {
		mapping, found := mappings[placementGroup]
		if !found {
			return fmt.Errorf("provider calendar %s has no %s mapping", syncState.ProviderCalendarID, placementGroup)
		}
		if placementError := placeProviderSeries(database, syncState.ID, providerSeriesID, mapping.CalendarID); placementError != nil {
			return placementError
		}
	}
	if boundsError := recalculateSyncStateLaneBounds(database, syncState.ID); boundsError != nil {
		return boundsError
	}
	if updateError := database.Model(syncState).Update("sync_cursor", batch.NextSyncCursor).Error; updateError != nil {
		return fmt.Errorf("store provider calendar event cursor: %w", updateError)
	}
	for _, mapping := range mappings {
		if orderError := NormalizeLaneOrder(database, mapping.CalendarID); orderError != nil {
			return orderError
		}
	}
	return nil
}

func providerSeriesPlacements(database *gorm.DB, syncStateID string, changes []ProviderEventChange, complete bool) (map[string]SemanticCalendarGroup, map[string]SemanticCalendarGroup, error) {
	var links []models.ExternalEventLink
	if findError := database.Where("sync_state_id = ?", syncStateID).Find(&links).Error; findError != nil {
		return nil, nil, findError
	}
	linksByEventID := make(map[string]models.ExternalEventLink, len(links))
	seriesByEventID := make(map[string]string, len(links))
	groupsByEventID := make(map[string]SemanticCalendarGroup, len(links))
	for _, link := range links {
		linksByEventID[link.ProviderEventID] = link
		if link.ProviderSeriesID == nil {
			continue
		}
		group := SemanticCalendarGroup(link.SemanticGroup)
		if group != SemanticCalendarGroupCalendar && group != SemanticCalendarGroupBirthdays {
			return nil, nil, errors.New("external event link has an unknown semantic group")
		}
		seriesByEventID[link.ProviderEventID] = *link.ProviderSeriesID
		groupsByEventID[link.ProviderEventID] = group
	}

	seen := make(map[string]struct{}, len(changes))
	affectedSeries := make(map[string]struct{})
	placements := make(map[string]SemanticCalendarGroup, len(changes))
	for _, change := range changes {
		if change.ProviderEventID == "" {
			return nil, nil, errors.New("provider event change has no source identity")
		}
		seen[change.ProviderEventID] = struct{}{}
		existingLink, exists := linksByEventID[change.ProviderEventID]
		existingSeriesID := ""
		if exists && existingLink.ProviderSeriesID != nil {
			existingSeriesID = *existingLink.ProviderSeriesID
		}
		seriesID := change.ProviderSeriesID
		if exists && seriesID != "" && seriesID != existingSeriesID {
			return nil, nil, errors.New("provider event series identity changed")
		}
		if seriesID == "" && change.Deleted {
			seriesID = existingSeriesID
		}
		if change.Deleted {
			delete(seriesByEventID, change.ProviderEventID)
			delete(groupsByEventID, change.ProviderEventID)
			if seriesID != "" {
				affectedSeries[seriesID] = struct{}{}
			}
			continue
		}
		if change.SemanticGroup != SemanticCalendarGroupCalendar && change.SemanticGroup != SemanticCalendarGroupBirthdays {
			return nil, nil, fmt.Errorf("provider event %s has an unknown semantic group", change.ProviderEventID)
		}
		placements[change.ProviderEventID] = change.SemanticGroup
		if seriesID != "" {
			seriesByEventID[change.ProviderEventID] = seriesID
			groupsByEventID[change.ProviderEventID] = change.SemanticGroup
			affectedSeries[seriesID] = struct{}{}
		}
	}
	if complete {
		for _, link := range links {
			if _, found := seen[link.ProviderEventID]; found {
				continue
			}
			if seriesID, found := seriesByEventID[link.ProviderEventID]; found {
				affectedSeries[seriesID] = struct{}{}
			}
			delete(seriesByEventID, link.ProviderEventID)
			delete(groupsByEventID, link.ProviderEventID)
		}
	}

	seriesGroups := make(map[string]SemanticCalendarGroup)
	for providerEventID, seriesID := range seriesByEventID {
		group := groupsByEventID[providerEventID]
		if group == SemanticCalendarGroupBirthdays {
			seriesGroups[seriesID] = group
			continue
		}
		if _, found := seriesGroups[seriesID]; !found {
			seriesGroups[seriesID] = group
		}
	}
	seriesPlacements := make(map[string]SemanticCalendarGroup, len(affectedSeries))
	for seriesID := range affectedSeries {
		if group, found := seriesGroups[seriesID]; found {
			seriesPlacements[seriesID] = group
		}
	}
	for _, change := range changes {
		if change.Deleted || change.ProviderSeriesID == "" {
			continue
		}
		placements[change.ProviderEventID] = seriesGroups[change.ProviderSeriesID]
	}
	return placements, seriesPlacements, nil
}

func recalculateSyncStateLaneBounds(database *gorm.DB, syncStateID string) error {
	var events []models.Event
	if findError := database.Model(&models.Event{}).
		Joins("JOIN external_event_links ON external_event_links.event_id = events.id AND external_event_links.deleted_at IS NULL").
		Where("external_event_links.sync_state_id = ?", syncStateID).Find(&events).Error; findError != nil {
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

func upsertExternalEvent(database *gorm.DB, syncStateID string, mapping *models.SourceCalendarMapping, startedAt time.Time, change ProviderEventChange) error {
	eventTime, boundsStart, boundsEnd, timeError := providerEventTime(change)
	if timeError != nil {
		return fmt.Errorf("convert source event time: %w", timeError)
	}
	var link models.ExternalEventLink
	findError := database.First(&link, "sync_state_id = ? AND provider_event_id = ?", syncStateID, change.ProviderEventID).Error
	if findError == nil {
		var event models.Event
		if eventError := database.First(&event, "id = ?", link.EventID).Error; eventError != nil {
			return eventError
		}
		currentSeriesID := ""
		if link.ProviderSeriesID != nil {
			currentSeriesID = *link.ProviderSeriesID
		}
		if currentSeriesID != change.ProviderSeriesID {
			return errors.New("provider event series identity changed")
		}
		candidate, candidateError := models.NewEvent(event.LaneID, change.Title, change.Description, nil, eventRelation(event), eventTime)
		if candidateError != nil {
			return fmt.Errorf("construct updated source event: %w", candidateError)
		}
		if placementError := placeSourceLane(database, event.LaneID, mapping.CalendarID, change.Title, startedAt, boundsStart, boundsEnd); placementError != nil {
			return placementError
		}
		if updateError := database.Model(&event).Updates(map[string]any{"title": candidate.Title, "description": candidate.Description, "time_shape": candidate.TimeShape, "at": candidate.At, "starts_at": candidate.StartsAt, "ends_at": candidate.EndsAt, "start_date": candidate.StartDate, "end_date": candidate.EndDate, "timezone": candidate.Timezone}).Error; updateError != nil {
			return updateError
		}
		if updateError := database.Model(&link).Updates(map[string]any{"semantic_group": models.SourceCalendarGroup(change.SemanticGroup), "diagnostic_code": diagnosticCode(change.DiagnosticCode)}).Error; updateError != nil {
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
	lane, series, laneError := sourceLane(database, syncStateID, mapping.CalendarID, startedAt, boundsStart, boundsEnd, change)
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
	event, eventError := models.NewEvent(lane.ID, change.Title, change.Description, nil, relation, eventTime)
	if eventError != nil {
		return fmt.Errorf("construct source event: %w", eventError)
	}
	if createError := database.Create(event).Error; createError != nil {
		return createError
	}
	var providerSeriesID *string
	if change.ProviderSeriesID != "" {
		providerSeriesID = &change.ProviderSeriesID
	}
	externalLink, externalError := models.NewExternalEventLink(syncStateID, event.ID, change.ProviderEventID, providerSeriesID, models.SourceCalendarGroup(change.SemanticGroup), diagnosticCode(change.DiagnosticCode))
	if externalError != nil {
		return externalError
	}
	return database.Create(externalLink).Error
}

func diagnosticCode(code string) *string {
	if code == "" {
		return nil
	}
	return &code
}

func placeProviderSeries(database *gorm.DB, syncStateID string, providerSeriesID string, calendarID string) error {
	var externalSeries models.ExternalEventSeriesLink
	findError := database.First(&externalSeries, "sync_state_id = ? AND provider_series_id = ?", syncStateID, providerSeriesID).Error
	if errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil
	}
	if findError != nil {
		return findError
	}
	var series models.EventSeries
	if findError := database.First(&series, "id = ?", externalSeries.EventSeriesID).Error; findError != nil {
		return findError
	}
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", series.LaneID).Error; findError != nil {
		return findError
	}
	if lane.CalendarID == calendarID {
		return nil
	}
	order, orderError := models.NextLaneDisplayOrder(database, calendarID)
	if orderError != nil {
		return orderError
	}
	return database.Model(&lane).Updates(map[string]any{"calendar_id": calendarID, "display_order": order}).Error
}

func sourceLane(database *gorm.DB, syncStateID string, calendarID string, startedAt time.Time, markerStart time.Time, markerEnd time.Time, change ProviderEventChange) (*models.Lane, *models.EventSeries, error) {
	if change.ProviderSeriesID != "" {
		var externalSeries models.ExternalEventSeriesLink
		findError := database.First(&externalSeries, "sync_state_id = ? AND provider_series_id = ?", syncStateID, change.ProviderSeriesID).Error
		if findError == nil {
			var series models.EventSeries
			if seriesError := database.First(&series, "id = ?", externalSeries.EventSeriesID).Error; seriesError != nil {
				return nil, nil, seriesError
			}
			if placementError := placeSourceLane(database, series.LaneID, calendarID, change.Title, startedAt, markerStart, markerEnd); placementError != nil {
				return nil, nil, placementError
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
	order, orderError := models.NextLaneDisplayOrder(database, calendarID)
	if orderError != nil {
		return nil, nil, orderError
	}
	lane, laneError := models.NewFiniteLane(calendarID, change.Title, start, end, order)
	if laneError != nil {
		return nil, nil, laneError
	}
	if createError := database.Create(lane).Error; createError != nil {
		return nil, nil, createError
	}
	if change.ProviderSeriesID == "" {
		return lane, nil, nil
	}
	timezone, timezoneError := models.NewTimezone(change.Timezone)
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
	externalSeries, externalError := models.NewExternalEventSeriesLink(syncStateID, series.ID, change.ProviderSeriesID)
	if externalError != nil {
		return nil, nil, externalError
	}
	if createError := database.Create(externalSeries).Error; createError != nil {
		return nil, nil, createError
	}
	return lane, series, nil
}

func placeSourceLane(database *gorm.DB, laneID string, calendarID string, title string, trackedAt time.Time, markerStart time.Time, markerEnd time.Time) error {
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
		return findError
	}
	updates := map[string]any{"title": title}
	if lane.CalendarID != calendarID {
		order, orderError := models.NextLaneDisplayOrder(database, calendarID)
		if orderError != nil {
			return orderError
		}
		updates["calendar_id"] = calendarID
		updates["display_order"] = order
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
	updates["starts_at"] = start
	updates["ends_at"] = end
	return database.Model(&lane).Updates(updates).Error
}

func providerEventTime(event ProviderEventChange) (models.EventTime, time.Time, time.Time, error) {
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

func deleteExternalEvent(database *gorm.DB, syncStateID string, providerEventID string) error {
	var link models.ExternalEventLink
	findError := database.First(&link, "sync_state_id = ? AND provider_event_id = ?", syncStateID, providerEventID).Error
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

func (service *CalendarSyncService) failSync(ctx context.Context, synchronization *models.CalendarSync, code string) error {
	finishedAt := service.now().UTC()
	synchronization.State, synchronization.FinishedAt, synchronization.ErrorCode = models.CalendarSyncFailed, &finishedAt, &code
	if saveError := service.database.WithContext(ctx).Save(synchronization).Error; saveError != nil {
		return fmt.Errorf("record failed calendar synchronization: %w", saveError)
	}
	return nil
}
