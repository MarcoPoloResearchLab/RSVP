package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrCalendarNotEmpty indicates that a calendar still owns a lane.
	ErrCalendarNotEmpty = errors.New("calendar has lanes")
	// ErrLaneHasRSVPs indicates that a lane contains an event with an RSVP.
	ErrLaneHasRSVPs = errors.New("lane has RSVPs")
	// ErrSourceOwnedLane indicates that calendar synchronization owns a lane.
	ErrSourceOwnedLane = errors.New("source-owned lane cannot be deleted directly")
	// ErrDisplayOrderOutOfRange indicates that an order position is outside its collection.
	ErrDisplayOrderOutOfRange = errors.New("display order is outside the collection")
	// ErrLaneResolutionInvalid indicates that a lane cannot use the requested resolution transition.
	ErrLaneResolutionInvalid = errors.New("only an active open lane can be resolved")
	// ErrLaneEndUpdateInvalid indicates that an active finite lane cannot use the requested end.
	ErrLaneEndUpdateInvalid = errors.New("finite lane end is invalid")
	// ErrResourceForbidden indicates that another organizer owns the addressed resource.
	ErrResourceForbidden = errors.New("another organizer owns the resource")
)

// CalendarPatch contains validated optional calendar changes.
type CalendarPatch struct {
	Name         *string
	Symbol       *string
	ColorToken   *string
	DisplayOrder *int
	Visible      *bool
}

// LanePatch contains validated optional lane changes.
type LanePatch struct {
	Title          *string
	CalendarID     *string
	DisplayOrder   *int
	EndsAt         *time.Time
	ResolutionTime *time.Time
}

// TemporalManagementService changes organizer-owned calendars and lanes.
type TemporalManagementService struct {
	database *gorm.DB
	now      func() time.Time
}

// NewTemporalManagementService constructs one temporal resource service.
func NewTemporalManagementService(database *gorm.DB, now func() time.Time) (*TemporalManagementService, error) {
	if database == nil || now == nil {
		return nil, errors.New("temporal management database and clock are required")
	}
	return &TemporalManagementService{database: database, now: now}, nil
}

// CreateCalendar creates one calendar at the end of the organizer calendar order.
func (service *TemporalManagementService) CreateCalendar(ctx context.Context, organizer *models.User, timezone models.Timezone, name string, symbol string, colorToken string) (*models.Calendar, error) {
	var createdCalendar *models.Calendar
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if confirmationError := organizer.ConfirmTimezone(transaction, timezone); confirmationError != nil {
			return confirmationError
		}
		displayOrder, orderError := models.NextCalendarDisplayOrder(transaction, organizer.ID)
		if orderError != nil {
			return orderError
		}
		calendar, calendarError := models.NewCalendar(organizer.ID, name, symbol, colorToken, displayOrder)
		if calendarError != nil {
			return calendarError
		}
		if createError := transaction.Create(calendar).Error; createError != nil {
			return fmt.Errorf("create calendar for organizer %s: %w", organizer.ID, createError)
		}
		createdCalendar = calendar
		return nil
	})
	return createdCalendar, transactionError
}

// ReadCalendar returns one organizer-owned calendar.
func (service *TemporalManagementService) ReadCalendar(ctx context.Context, organizerID string, calendarID string) (*models.Calendar, error) {
	var calendar models.Calendar
	if findError := service.database.WithContext(ctx).First(&calendar, "id = ?", calendarID).Error; findError != nil {
		return nil, findError
	}
	if calendar.OrganizerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &calendar, nil
}

// UpdateCalendar changes one organizer-owned calendar.
func (service *TemporalManagementService) UpdateCalendar(ctx context.Context, organizerID string, calendarID string, patch CalendarPatch) (*models.Calendar, error) {
	var updatedCalendar models.Calendar
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updatedCalendar, "id = ?", calendarID).Error; findError != nil {
			return findError
		}
		if updatedCalendar.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		if patch.DisplayOrder != nil {
			if reorderError := reorderCalendars(transaction, organizerID, calendarID, *patch.DisplayOrder); reorderError != nil {
				return reorderError
			}
		}
		if patch.Name != nil {
			updatedCalendar.Name = strings.TrimSpace(*patch.Name)
		}
		if patch.Symbol != nil {
			updatedCalendar.Symbol = strings.TrimSpace(*patch.Symbol)
		}
		if patch.ColorToken != nil {
			updatedCalendar.ColorToken = strings.TrimSpace(*patch.ColorToken)
		}
		if patch.Visible != nil {
			updatedCalendar.Visible = *patch.Visible
		}
		if validationError := updatedCalendar.Validate(); validationError != nil {
			return validationError
		}
		if updateError := transaction.Model(&updatedCalendar).Updates(map[string]any{
			"name": updatedCalendar.Name, "symbol": updatedCalendar.Symbol,
			"color_token": updatedCalendar.ColorToken, "visible": updatedCalendar.Visible,
		}).Error; updateError != nil {
			return fmt.Errorf("update calendar %s: %w", calendarID, updateError)
		}
		return transaction.First(&updatedCalendar, "id = ?", calendarID).Error
	})
	return &updatedCalendar, transactionError
}

// DeleteCalendar deletes one empty organizer-owned calendar.
func (service *TemporalManagementService) DeleteCalendar(ctx context.Context, organizerID string, calendarID string) error {
	return service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var calendar models.Calendar
		if findError := transaction.First(&calendar, "id = ?", calendarID).Error; findError != nil {
			return findError
		}
		if calendar.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		var laneCount int64
		if countError := transaction.Model(&models.Lane{}).Where("calendar_id = ?", calendarID).Count(&laneCount).Error; countError != nil {
			return fmt.Errorf("count lanes for calendar %s: %w", calendarID, countError)
		}
		if laneCount != 0 {
			return ErrCalendarNotEmpty
		}
		if deleteError := transaction.Unscoped().Delete(&calendar).Error; deleteError != nil {
			return fmt.Errorf("delete calendar %s: %w", calendarID, deleteError)
		}
		return normalizeCalendarOrder(transaction, organizerID)
	})
}

// CreateLane creates one open or finite lane at the request reference time.
func (service *TemporalManagementService) CreateLane(ctx context.Context, organizer *models.User, timezone models.Timezone, calendarID string, title string, endsAt *time.Time) (*models.Lane, error) {
	var createdLane *models.Lane
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if confirmationError := organizer.ConfirmTimezone(transaction, timezone); confirmationError != nil {
			return confirmationError
		}
		if ownerError := requireCalendarOwner(transaction, calendarID, organizer.ID); ownerError != nil {
			return ownerError
		}
		displayOrder, orderError := models.NextLaneDisplayOrder(transaction, calendarID)
		if orderError != nil {
			return orderError
		}
		referenceTime := service.now().UTC()
		var lane *models.Lane
		var laneError error
		if endsAt == nil {
			lane, laneError = models.NewOpenLane(calendarID, title, referenceTime, displayOrder)
		} else {
			lane, laneError = models.NewFiniteLane(calendarID, title, referenceTime, *endsAt, displayOrder)
		}
		if laneError != nil {
			return laneError
		}
		if createError := transaction.Create(lane).Error; createError != nil {
			return fmt.Errorf("create lane in calendar %s: %w", calendarID, createError)
		}
		createdLane = lane
		return nil
	})
	return createdLane, transactionError
}

// ReadLane returns one organizer-owned lane and its resource relationships.
func (service *TemporalManagementService) ReadLane(ctx context.Context, organizerID string, laneID string) (*models.Lane, error) {
	var lane models.Lane
	if findError := service.database.WithContext(ctx).Preload("Events").Preload("Events.RSVPs").
		First(&lane, "id = ?", laneID).Error; findError != nil {
		return nil, findError
	}
	if ownerError := requireCalendarOwner(service.database.WithContext(ctx), lane.CalendarID, organizerID); ownerError != nil {
		return nil, ownerError
	}
	return &lane, nil
}

// UpdateLane changes, moves, reorders, or resolves one organizer-owned lane.
func (service *TemporalManagementService) UpdateLane(ctx context.Context, organizerID string, laneID string, patch LanePatch) (*models.Lane, error) {
	var updatedLane models.Lane
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updatedLane, "id = ?", laneID).Error; findError != nil {
			return findError
		}
		if ownerError := requireCalendarOwner(transaction, updatedLane.CalendarID, organizerID); ownerError != nil {
			return ownerError
		}
		targetCalendarID := updatedLane.CalendarID
		if patch.CalendarID != nil {
			targetCalendarID = *patch.CalendarID
			if ownerError := requireCalendarOwner(transaction, targetCalendarID, organizerID); ownerError != nil {
				return ownerError
			}
		}
		if targetCalendarID != updatedLane.CalendarID || patch.DisplayOrder != nil {
			targetOrder := -1
			if patch.DisplayOrder != nil {
				targetOrder = *patch.DisplayOrder
			}
			if reorderError := moveAndReorderLane(transaction, &updatedLane, targetCalendarID, targetOrder); reorderError != nil {
				return reorderError
			}
		}
		if patch.Title != nil {
			updatedLane.Title = strings.TrimSpace(*patch.Title)
		}
		if patch.EndsAt != nil {
			if updatedLane.Status != models.LaneStatusActive || updatedLane.EndsAt == nil {
				return ErrLaneEndUpdateInvalid
			}
			canonicalEnd := patch.EndsAt.UTC()
			if !canonicalEnd.After(updatedLane.StartsAt) {
				return ErrLaneEndUpdateInvalid
			}
			lastMarkerEnd, markerFound, markerError := lastEventBoundary(transaction, updatedLane.ID)
			if markerError != nil {
				return markerError
			}
			if markerFound && canonicalEnd.Before(lastMarkerEnd) {
				return models.ErrMarkerOutsideLane
			}
			updatedLane.EndsAt = &canonicalEnd
		}
		if patch.ResolutionTime != nil {
			if updatedLane.Status != models.LaneStatusActive || updatedLane.EndsAt != nil {
				return ErrLaneResolutionInvalid
			}
			resolutionTime := patch.ResolutionTime.UTC()
			if !resolutionTime.After(updatedLane.StartsAt) {
				return ErrLaneResolutionInvalid
			}
			lastMarkerEnd, markerFound, markerError := lastEventBoundary(transaction, updatedLane.ID)
			if markerError != nil {
				return markerError
			}
			if markerFound && resolutionTime.Before(lastMarkerEnd) {
				return models.ErrMarkerOutsideLane
			}
			updatedLane.Status = models.LaneStatusResolved
			updatedLane.EndsAt = &resolutionTime
			updatedLane.ResolvedAt = &resolutionTime
			if cancelError := transaction.Model(&models.Probe{}).
				Where("lane_id = ? AND state = ?", laneID, models.ProbeStatePending).
				UpdateColumns(map[string]any{"state": models.ProbeStateCanceled, "completed_at": nil}).Error; cancelError != nil {
				return fmt.Errorf("cancel pending probes for lane %s: %w", laneID, cancelError)
			}
		}
		if validationError := updatedLane.Validate(); validationError != nil {
			return validationError
		}
		if updateError := transaction.Model(&updatedLane).Updates(map[string]any{
			"title": updatedLane.Title, "status": updatedLane.Status,
			"ends_at": updatedLane.EndsAt, "resolved_at": updatedLane.ResolvedAt,
		}).Error; updateError != nil {
			return fmt.Errorf("update lane %s: %w", laneID, updateError)
		}
		return transaction.First(&updatedLane, "id = ?", laneID).Error
	})
	return &updatedLane, transactionError
}

// DeleteLane deletes one eligible local lane and all of its temporal children.
func (service *TemporalManagementService) DeleteLane(ctx context.Context, organizerID string, laneID string) error {
	return service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lane models.Lane
		if findError := transaction.First(&lane, "id = ?", laneID).Error; findError != nil {
			return findError
		}
		if ownerError := requireCalendarOwner(transaction, lane.CalendarID, organizerID); ownerError != nil {
			return ownerError
		}
		var sourceSeriesCount int64
		if countError := transaction.Model(&models.EventSeries{}).
			Where("lane_id = ? AND source_kind <> ?", laneID, models.EventSourceLocal).
			Count(&sourceSeriesCount).Error; countError != nil {
			return fmt.Errorf("inspect lane %s source ownership: %w", laneID, countError)
		}
		if sourceSeriesCount != 0 {
			return ErrSourceOwnedLane
		}
		var rsvpCount int64
		if countError := transaction.Model(&models.RSVP{}).
			Joins("JOIN "+config.TableEvents+" ON "+config.TableEvents+".id = "+config.TableRSVPs+".event_id AND "+config.TableEvents+".deleted_at IS NULL").
			Where(config.TableEvents+".lane_id = ?", laneID).Count(&rsvpCount).Error; countError != nil {
			return fmt.Errorf("count RSVPs for lane %s: %w", laneID, countError)
		}
		if rsvpCount != 0 {
			return ErrLaneHasRSVPs
		}
		if deleteError := deleteLaneChildren(transaction, laneID); deleteError != nil {
			return deleteError
		}
		if deleteError := transaction.Unscoped().Delete(&lane).Error; deleteError != nil {
			return fmt.Errorf("delete lane %s: %w", laneID, deleteError)
		}
		return NormalizeLaneOrder(transaction, lane.CalendarID)
	})
}

func requireCalendarOwner(database *gorm.DB, calendarID string, organizerID string) error {
	var calendar models.Calendar
	if findError := database.First(&calendar, "id = ?", calendarID).Error; findError != nil {
		return fmt.Errorf("find calendar %s: %w", calendarID, findError)
	}
	if calendar.OrganizerID != organizerID {
		return ErrResourceForbidden
	}
	return nil
}

func orderedCalendarIDs(database *gorm.DB, organizerID string) ([]string, error) {
	var identifiers []string
	errorValue := database.Model(&models.Calendar{}).Where("organizer_id = ?", organizerID).
		Order("display_order ASC").Order("id ASC").Pluck("id", &identifiers).Error
	return identifiers, errorValue
}

func orderedLaneIDs(database *gorm.DB, calendarID string) ([]string, error) {
	var identifiers []string
	errorValue := database.Model(&models.Lane{}).Where("calendar_id = ?", calendarID).
		Order("display_order ASC").Order("id ASC").Pluck("id", &identifiers).Error
	return identifiers, errorValue
}

func reorderCalendars(database *gorm.DB, organizerID string, calendarID string, targetOrder int) error {
	identifiers, queryError := orderedCalendarIDs(database, organizerID)
	if queryError != nil {
		return queryError
	}
	return reorderIdentifiers(database, &models.Calendar{}, "organizer_id", organizerID, identifiers, calendarID, targetOrder)
}

func normalizeCalendarOrder(database *gorm.DB, organizerID string) error {
	identifiers, queryError := orderedCalendarIDs(database, organizerID)
	if queryError != nil {
		return queryError
	}
	return assignOrders(database, &models.Calendar{}, "organizer_id", organizerID, identifiers)
}

// NormalizeLaneOrder assigns contiguous active lane positions in one calendar.
func NormalizeLaneOrder(database *gorm.DB, calendarID string) error {
	identifiers, queryError := orderedLaneIDs(database, calendarID)
	if queryError != nil {
		return queryError
	}
	return assignOrders(database, &models.Lane{}, "calendar_id", calendarID, identifiers)
}

func reorderIdentifiers(database *gorm.DB, model any, ownerColumn string, ownerID string, identifiers []string, targetID string, targetOrder int) error {
	if targetOrder < 0 || targetOrder >= len(identifiers) {
		return ErrDisplayOrderOutOfRange
	}
	ordered := make([]string, 0, len(identifiers))
	found := false
	for _, identifier := range identifiers {
		if identifier == targetID {
			found = true
			continue
		}
		ordered = append(ordered, identifier)
	}
	if !found {
		return gorm.ErrRecordNotFound
	}
	ordered = append(ordered, "")
	copy(ordered[targetOrder+1:], ordered[targetOrder:])
	ordered[targetOrder] = targetID
	return assignOrders(database, model, ownerColumn, ownerID, ordered)
}

func assignOrders(database *gorm.DB, model any, ownerColumn string, ownerID string, identifiers []string) error {
	if len(identifiers) == 0 {
		return nil
	}
	var maximum int
	if maximumError := database.Unscoped().Model(model).Select("COALESCE(MAX(display_order), 0)").Scan(&maximum).Error; maximumError != nil {
		return maximumError
	}
	offset := maximum + len(identifiers) + 1
	if shiftError := database.Unscoped().Model(model).Where(ownerColumn+" = ?", ownerID).
		UpdateColumn("display_order", gorm.Expr("display_order + ?", offset)).Error; shiftError != nil {
		return shiftError
	}
	for order, identifier := range identifiers {
		if updateError := database.Model(model).Where("id = ?", identifier).UpdateColumn("display_order", order).Error; updateError != nil {
			return updateError
		}
	}
	return nil
}

func moveAndReorderLane(database *gorm.DB, lane *models.Lane, targetCalendarID string, targetOrder int) error {
	oldCalendarID := lane.CalendarID
	oldIDs, oldError := orderedLaneIDs(database, oldCalendarID)
	if oldError != nil {
		return oldError
	}
	oldWithoutTarget := make([]string, 0, len(oldIDs)-1)
	for _, identifier := range oldIDs {
		if identifier != lane.ID {
			oldWithoutTarget = append(oldWithoutTarget, identifier)
		}
	}
	if targetCalendarID == oldCalendarID {
		if targetOrder < 0 {
			targetOrder = len(oldIDs) - 1
		}
		if reorderError := reorderIdentifiers(database, &models.Lane{}, "calendar_id", oldCalendarID, oldIDs, lane.ID, targetOrder); reorderError != nil {
			return reorderError
		}
		return database.First(lane, "id = ?", lane.ID).Error
	}
	newIDs, newError := orderedLaneIDs(database, targetCalendarID)
	if newError != nil {
		return newError
	}
	if targetOrder < 0 {
		targetOrder = len(newIDs)
	}
	if targetOrder < 0 || targetOrder > len(newIDs) {
		return ErrDisplayOrderOutOfRange
	}
	if assignError := assignOrders(database, &models.Lane{}, "calendar_id", oldCalendarID, oldIDs); assignError != nil {
		return assignError
	}
	if assignError := assignOrders(database, &models.Lane{}, "calendar_id", targetCalendarID, newIDs); assignError != nil {
		return assignError
	}
	temporaryOrder := len(newIDs) + len(oldIDs) + 1
	if moveError := database.Model(&models.Lane{}).Where("id = ?", lane.ID).
		UpdateColumns(map[string]any{"calendar_id": targetCalendarID, "display_order": temporaryOrder}).Error; moveError != nil {
		return moveError
	}
	newIDs = append(newIDs, "")
	copy(newIDs[targetOrder+1:], newIDs[targetOrder:])
	newIDs[targetOrder] = lane.ID
	if assignError := assignOrders(database, &models.Lane{}, "calendar_id", oldCalendarID, oldWithoutTarget); assignError != nil {
		return assignError
	}
	if assignError := assignOrders(database, &models.Lane{}, "calendar_id", targetCalendarID, newIDs); assignError != nil {
		return assignError
	}
	lane.CalendarID = targetCalendarID
	return database.First(lane, "id = ?", lane.ID).Error
}

func lastEventBoundary(database *gorm.DB, laneID string) (time.Time, bool, error) {
	var events []models.Event
	if findError := database.Where("lane_id = ?", laneID).Find(&events).Error; findError != nil {
		return time.Time{}, false, findError
	}
	var lastBoundary time.Time
	for eventIndex := range events {
		_, eventEnd, boundsError := events[eventIndex].MarkerBounds()
		if boundsError != nil {
			return time.Time{}, false, boundsError
		}
		if eventEnd.After(lastBoundary) {
			lastBoundary = eventEnd
		}
	}
	return lastBoundary, !lastBoundary.IsZero(), nil
}

func deleteLaneChildren(database *gorm.DB, laneID string) error {
	if deleteError := database.Unscoped().Where("lane_id = ?", laneID).Delete(&models.Probe{}).Error; deleteError != nil {
		return fmt.Errorf("delete probes for lane %s: %w", laneID, deleteError)
	}
	if deleteError := database.Unscoped().Where("lane_id = ?", laneID).Delete(&models.AttentionPolicy{}).Error; deleteError != nil {
		return fmt.Errorf("delete attention policies for lane %s: %w", laneID, deleteError)
	}
	var events []models.Event
	if findError := database.Unscoped().Where("lane_id = ?", laneID).
		Order("CASE relation_type WHEN 'dependent' THEN 0 ELSE 1 END").Find(&events).Error; findError != nil {
		return fmt.Errorf("read events for lane %s deletion: %w", laneID, findError)
	}
	for eventIndex := range events {
		if deleteError := database.Unscoped().Delete(&events[eventIndex]).Error; deleteError != nil {
			return fmt.Errorf("delete event %s for lane %s: %w", events[eventIndex].ID, laneID, deleteError)
		}
	}
	if deleteError := database.Unscoped().Where("lane_id = ?", laneID).Delete(&models.EventSeries{}).Error; deleteError != nil {
		return fmt.Errorf("delete event series for lane %s: %w", laneID, deleteError)
	}
	return nil
}
