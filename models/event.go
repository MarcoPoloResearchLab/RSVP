package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// EventRelationType identifies one event-to-lane relationship.
type EventRelationType string

const (
	EventRelationIndependent      EventRelationType = "independent"
	EventRelationSeriesOccurrence EventRelationType = "series_occurrence"
	EventRelationDependent        EventRelationType = "dependent"
)

// EventTimeShape identifies one closed event time representation.
type EventTimeShape string

const (
	EventTimePoint    EventTimeShape = "point"
	EventTimeInterval EventTimeShape = "interval"
	EventTimeAllDay   EventTimeShape = "all_day"
)

const (
	eventLaneJoin     = "JOIN " + config.TableLanes + " ON " + config.TableLanes + ".id = " + config.TableEvents + ".lane_id AND " + config.TableLanes + ".deleted_at IS NULL"
	eventCalendarJoin = "JOIN " + config.TableCalendars + " ON " + config.TableCalendars + ".id = " + config.TableLanes + ".calendar_id AND " + config.TableCalendars + ".deleted_at IS NULL"
)

var (
	ErrEventRelationInvalid   = errors.New("event relation is invalid")
	ErrEventTimeInvalid       = errors.New("event time is invalid")
	ErrEventMembershipInvalid = errors.New("event lane membership is invalid")
	ErrEventHasDependents     = errors.New("event has dependent events")
)

// EventRelation is a validated event membership proposal.
type EventRelation struct {
	relationType  EventRelationType
	eventSeriesID *string
	anchorEventID *string
}

// IndependentEventRelation constructs independent membership.
func IndependentEventRelation() EventRelation {
	return EventRelation{relationType: EventRelationIndependent}
}

// SeriesOccurrenceRelation constructs event series membership.
func SeriesOccurrenceRelation(eventSeriesID string) (EventRelation, error) {
	if eventSeriesID == "" {
		return EventRelation{}, ErrEventSeriesIDRequired
	}
	return EventRelation{relationType: EventRelationSeriesOccurrence, eventSeriesID: &eventSeriesID}, nil
}

// DependentEventRelation constructs dependency-chain membership.
func DependentEventRelation(anchorEventID string) (EventRelation, error) {
	if anchorEventID == "" {
		return EventRelation{}, ErrEventRelationInvalid
	}
	return EventRelation{relationType: EventRelationDependent, anchorEventID: &anchorEventID}, nil
}

// EventTime is a validated event time proposal.
type EventTime struct {
	shape     EventTimeShape
	at        *time.Time
	startsAt  *time.Time
	endsAt    *time.Time
	startDate *string
	endDate   *string
	timezone  Timezone
}

// NewPointEventTime constructs one point time.
func NewPointEventTime(at time.Time, timezone Timezone) (EventTime, error) {
	canonicalAt := at.UTC()
	eventTime := EventTime{shape: EventTimePoint, at: &canonicalAt, timezone: timezone}
	return eventTime, eventTime.Validate()
}

// NewIntervalEventTime constructs one interval time.
func NewIntervalEventTime(startsAt time.Time, endsAt time.Time, timezone Timezone) (EventTime, error) {
	canonicalStart := startsAt.UTC()
	canonicalEnd := endsAt.UTC()
	eventTime := EventTime{shape: EventTimeInterval, startsAt: &canonicalStart, endsAt: &canonicalEnd, timezone: timezone}
	return eventTime, eventTime.Validate()
}

// NewAllDayEventTime constructs one all-day local-date range.
func NewAllDayEventTime(startDate LocalDate, endDate LocalDate, timezone Timezone) (EventTime, error) {
	canonicalStart := startDate.String()
	canonicalEnd := endDate.String()
	eventTime := EventTime{shape: EventTimeAllDay, startDate: &canonicalStart, endDate: &canonicalEnd, timezone: timezone}
	return eventTime, eventTime.Validate()
}

// Validate checks one event time proposal.
func (eventTime EventTime) Validate() error {
	if _, timezoneError := NewTimezone(eventTime.timezone.String()); timezoneError != nil {
		return timezoneError
	}
	switch eventTime.shape {
	case EventTimePoint:
		if eventTime.at == nil || eventTime.at.IsZero() || eventTime.startsAt != nil || eventTime.endsAt != nil || eventTime.startDate != nil || eventTime.endDate != nil {
			return ErrEventTimeInvalid
		}
	case EventTimeInterval:
		if eventTime.startsAt == nil || eventTime.endsAt == nil || eventTime.startsAt.IsZero() || !eventTime.endsAt.After(*eventTime.startsAt) || eventTime.at != nil || eventTime.startDate != nil || eventTime.endDate != nil {
			return ErrEventTimeInvalid
		}
	case EventTimeAllDay:
		if eventTime.startDate == nil || eventTime.endDate == nil || *eventTime.endDate <= *eventTime.startDate || eventTime.at != nil || eventTime.startsAt != nil || eventTime.endsAt != nil {
			return ErrEventTimeInvalid
		}
		if _, startError := NewLocalDate(*eventTime.startDate); startError != nil {
			return startError
		}
		if _, endError := NewLocalDate(*eventTime.endDate); endError != nil {
			return endError
		}
	default:
		return ErrEventTimeInvalid
	}
	return nil
}

// Event represents one point, interval, or all-day marker on a lane.
type Event struct {
	BaseModel
	LaneID        string            `gorm:"type:varchar(8);not null;index"`
	EventSeriesID *string           `gorm:"type:varchar(8);index"`
	AnchorEventID *string           `gorm:"type:varchar(8);index"`
	RelationType  EventRelationType `gorm:"type:text;not null;check:event_relation,((relation_type = 'independent' AND event_series_id IS NULL AND anchor_event_id IS NULL) OR (relation_type = 'series_occurrence' AND event_series_id IS NOT NULL AND anchor_event_id IS NULL) OR (relation_type = 'dependent' AND event_series_id IS NULL AND anchor_event_id IS NOT NULL))"`
	TimeShape     EventTimeShape    `gorm:"type:text;not null;check:event_time_shape,((time_shape = 'point' AND at IS NOT NULL AND starts_at IS NULL AND ends_at IS NULL AND start_date IS NULL AND end_date IS NULL) OR (time_shape = 'interval' AND at IS NULL AND starts_at IS NOT NULL AND ends_at > starts_at AND start_date IS NULL AND end_date IS NULL) OR (time_shape = 'all_day' AND at IS NULL AND starts_at IS NULL AND ends_at IS NULL AND start_date IS NOT NULL AND end_date > start_date))"`
	At            *time.Time
	StartsAt      *time.Time
	EndsAt        *time.Time
	StartDate     *string `gorm:"type:text"`
	EndDate       *string `gorm:"type:text"`
	Timezone      string  `gorm:"type:text;not null;check:event_timezone,timezone <> ''"`
	Title         string  `gorm:"not null"`
	Description   string
	VenueID       *string      `gorm:"type:varchar(8);index"`
	RSVPs         []RSVP       `gorm:"foreignKey:EventID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lane          Lane         `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	EventSeries   *EventSeries `gorm:"foreignKey:EventSeriesID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	AnchorEvent   *Event       `gorm:"foreignKey:AnchorEventID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Venue         *Venue       `gorm:"foreignKey:VenueID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

// NewEvent constructs one valid event before persistence.
func NewEvent(laneID string, title string, description string, venueID *string, relation EventRelation, eventTime EventTime) (*Event, error) {
	event := &Event{
		LaneID:        laneID,
		EventSeriesID: relation.eventSeriesID,
		AnchorEventID: relation.anchorEventID,
		RelationType:  relation.relationType,
		TimeShape:     eventTime.shape,
		At:            eventTime.at,
		StartsAt:      eventTime.startsAt,
		EndsAt:        eventTime.endsAt,
		StartDate:     eventTime.startDate,
		EndDate:       eventTime.endDate,
		Timezone:      eventTime.timezone.String(),
		Title:         title,
		Description:   description,
		VenueID:       venueID,
	}
	if validationError := event.Validate(); validationError != nil {
		return nil, validationError
	}
	return event, nil
}

// Validate checks the event state without database relationships.
func (event *Event) Validate() error {
	if event.LaneID == "" {
		return ErrLaneIDRequired
	}
	if titleError := utils.ValidateEventTitle(event.Title); titleError != nil {
		return titleError
	}
	relation := EventRelation{relationType: event.RelationType, eventSeriesID: event.EventSeriesID, anchorEventID: event.AnchorEventID}
	if relationError := relation.validate(); relationError != nil {
		return relationError
	}
	timeValue := EventTime{shape: event.TimeShape, at: event.At, startsAt: event.StartsAt, endsAt: event.EndsAt, startDate: event.StartDate, endDate: event.EndDate, timezone: Timezone(event.Timezone)}
	return timeValue.Validate()
}

func (relation EventRelation) validate() error {
	switch relation.relationType {
	case EventRelationIndependent:
		if relation.eventSeriesID != nil || relation.anchorEventID != nil {
			return ErrEventRelationInvalid
		}
	case EventRelationSeriesOccurrence:
		if relation.eventSeriesID == nil || *relation.eventSeriesID == "" || relation.anchorEventID != nil {
			return ErrEventRelationInvalid
		}
	case EventRelationDependent:
		if relation.anchorEventID == nil || *relation.anchorEventID == "" || relation.eventSeriesID != nil {
			return ErrEventRelationInvalid
		}
	default:
		return ErrEventRelationInvalid
	}
	return nil
}

// MarkerBounds returns the UTC boundaries that must stay within the lane.
func (event *Event) MarkerBounds() (time.Time, time.Time, error) {
	switch event.TimeShape {
	case EventTimePoint:
		if event.At == nil {
			return time.Time{}, time.Time{}, ErrEventTimeInvalid
		}
		return event.At.UTC(), event.At.UTC(), nil
	case EventTimeInterval:
		if event.StartsAt == nil || event.EndsAt == nil {
			return time.Time{}, time.Time{}, ErrEventTimeInvalid
		}
		return event.StartsAt.UTC(), event.EndsAt.UTC(), nil
	case EventTimeAllDay:
		if event.StartDate == nil || event.EndDate == nil {
			return time.Time{}, time.Time{}, ErrEventTimeInvalid
		}
		timezone, timezoneError := NewTimezone(event.Timezone)
		if timezoneError != nil {
			return time.Time{}, time.Time{}, timezoneError
		}
		location, locationError := timezone.Location()
		if locationError != nil {
			return time.Time{}, time.Time{}, locationError
		}
		start, startError := time.ParseInLocation(time.DateOnly, *event.StartDate, location)
		if startError != nil {
			return time.Time{}, time.Time{}, startError
		}
		end, endError := time.ParseInLocation(time.DateOnly, *event.EndDate, location)
		if endError != nil {
			return time.Time{}, time.Time{}, endError
		}
		return start.UTC(), end.UTC(), nil
	default:
		return time.Time{}, time.Time{}, ErrEventTimeInvalid
	}
}

func (event *Event) validateRelationships(databaseConnection *gorm.DB) error {
	var lane Lane
	if findError := databaseConnection.First(&lane, "id = ?", event.LaneID).Error; findError != nil {
		return fmt.Errorf("find lane %s for event: %w", event.LaneID, findError)
	}
	markerStart, markerEnd, boundsError := event.MarkerBounds()
	if boundsError != nil {
		return boundsError
	}
	if markerStart.Before(lane.StartsAt) || (lane.EndsAt != nil && markerEnd.After(*lane.EndsAt)) {
		return ErrMarkerOutsideLane
	}
	var conflictingCount int64
	query := databaseConnection.Model(&Event{}).Where("lane_id = ? AND id <> ?", event.LaneID, event.ID)
	switch event.RelationType {
	case EventRelationIndependent:
		if countError := query.Where("relation_type <> ? OR anchor_event_id <> ?", EventRelationDependent, event.ID).Count(&conflictingCount).Error; countError != nil {
			return countError
		}
		if conflictingCount != 0 {
			return ErrEventMembershipInvalid
		}
	case EventRelationSeriesOccurrence:
		var series EventSeries
		if findError := databaseConnection.First(&series, "id = ?", *event.EventSeriesID).Error; findError != nil {
			return findError
		}
		if series.LaneID != event.LaneID {
			return ErrEventMembershipInvalid
		}
		if countError := query.Where("relation_type <> ? OR event_series_id <> ?", EventRelationSeriesOccurrence, *event.EventSeriesID).Count(&conflictingCount).Error; countError != nil {
			return countError
		}
		if conflictingCount != 0 {
			return ErrEventMembershipInvalid
		}
	case EventRelationDependent:
		var anchor Event
		if findError := databaseConnection.First(&anchor, "id = ?", *event.AnchorEventID).Error; findError != nil {
			return findError
		}
		if anchor.LaneID != event.LaneID || anchor.RelationType == EventRelationDependent {
			return ErrEventMembershipInvalid
		}
		if countError := query.Where("id <> ? AND (relation_type <> ? OR anchor_event_id <> ?)", anchor.ID, EventRelationDependent, anchor.ID).Count(&conflictingCount).Error; countError != nil {
			return countError
		}
		if conflictingCount != 0 {
			return ErrEventMembershipInvalid
		}
	}
	return nil
}

func (event *Event) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := event.Validate(); validationError != nil {
		return validationError
	}
	if idError := event.BaseModel.GenerateID(databaseConnection, event); idError != nil {
		return idError
	}
	return event.validateRelationships(databaseConnection)
}

func (event *Event) BeforeUpdate(databaseConnection *gorm.DB) error {
	if validationError := event.Validate(); validationError != nil {
		return validationError
	}
	return event.validateRelationships(databaseConnection)
}

func (event *Event) GetTableName() string                          { return config.TableEvents }
func (event *Event) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }

// LocalMarkerBounds returns marker bounds in the event timezone.
func (event *Event) LocalMarkerBounds() (time.Time, time.Time, error) {
	start, end, boundsError := event.MarkerBounds()
	if boundsError != nil {
		return time.Time{}, time.Time{}, boundsError
	}
	timezone, timezoneError := NewTimezone(event.Timezone)
	if timezoneError != nil {
		return time.Time{}, time.Time{}, timezoneError
	}
	location, locationError := timezone.Location()
	if locationError != nil {
		return time.Time{}, time.Time{}, locationError
	}
	return start.In(location), end.In(location), nil
}

// IntervalDurationHours returns an interval event duration in whole hours.
func (event *Event) IntervalDurationHours() (int, error) {
	if event.TimeShape != EventTimeInterval || event.StartsAt == nil || event.EndsAt == nil {
		return 0, ErrEventTimeInvalid
	}
	return int(event.EndsAt.Sub(*event.StartsAt).Hours()), nil
}

// SetIntervalTime replaces an event time with one validated interval.
func (event *Event) SetIntervalTime(startsAt time.Time, endsAt time.Time, timezone Timezone) error {
	eventTime, eventTimeError := NewIntervalEventTime(startsAt, endsAt, timezone)
	if eventTimeError != nil {
		return eventTimeError
	}
	event.TimeShape = eventTime.shape
	event.At = nil
	event.StartsAt = eventTime.startsAt
	event.EndsAt = eventTime.endsAt
	event.StartDate = nil
	event.EndDate = nil
	event.Timezone = eventTime.timezone.String()
	return nil
}

func (event *Event) FindByID(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Where("id = ?", eventIdentifier).First(event).Error
}

func (event *Event) FindByIDAndOwner(databaseConnection *gorm.DB, eventIdentifier string, ownerUserID string) error {
	return databaseConnection.Preload("Venue").Preload("Lane").
		Joins(eventLaneJoin).
		Joins(eventCalendarJoin).
		Where("events.id = ? AND calendars.organizer_id = ?", eventIdentifier, ownerUserID).
		First(event).Error
}

// OwnerID returns the organizer identifier through the event relationship chain.
func (event *Event) OwnerID(databaseConnection *gorm.DB) (string, error) {
	var ownerID string
	ownerQueryError := databaseConnection.Table(config.TableEvents).
		Select("calendars.organizer_id").
		Joins(eventLaneJoin).
		Joins(eventCalendarJoin).
		Where("events.id = ? AND events.deleted_at IS NULL", event.ID).
		Scan(&ownerID).Error
	if ownerQueryError != nil {
		return "", fmt.Errorf("find owner for event %s: %w", event.ID, ownerQueryError)
	}
	if ownerID == "" {
		return "", gorm.ErrRecordNotFound
	}
	return ownerID, nil
}

// HasDependentEvents reports whether an anchor event has active dependents.
func (event *Event) HasDependentEvents(databaseConnection *gorm.DB) (bool, error) {
	var dependentCount int64
	if countError := databaseConnection.Model(&Event{}).
		Where("anchor_event_id = ?", event.ID).
		Count(&dependentCount).Error; countError != nil {
		return false, fmt.Errorf("count dependent events for anchor %s: %w", event.ID, countError)
	}
	return dependentCount != 0, nil
}

func FindEventsByUserID(databaseConnection *gorm.DB, ownerUserID string, preloadRSVPs bool, preloadVenues bool) ([]Event, error) {
	var events []Event
	query := databaseConnection.
		Preload("Lane").
		Joins(eventLaneJoin).
		Joins(eventCalendarJoin).
		Where("calendars.organizer_id = ?", ownerUserID).
		Order("events.starts_at DESC")
	if preloadRSVPs {
		query = query.Preload("RSVPs")
	}
	if preloadVenues {
		query = query.Preload("Venue")
	}
	return events, query.Find(&events).Error
}

func (event *Event) Create(databaseConnection *gorm.DB) error {
	if event.VenueID == nil {
		event.Venue = nil
	}
	return databaseConnection.Create(event).Error
}

func (event *Event) Update(databaseConnection *gorm.DB) error {
	if event.VenueID == nil {
		event.Venue = nil
	}
	return databaseConnection.Save(event).Error
}

func (event *Event) LoadWithRSVPs(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Preload("RSVPs").Where("id = ?", eventIdentifier).First(event).Error
}

func (event *Event) LoadWithVenue(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Preload("Venue").Where("id = ?", eventIdentifier).First(event).Error
}

func FindVenueIDsAssociatedWithUserEvents(databaseConnection *gorm.DB, ownerUserID string) ([]string, error) {
	var identifiers []string
	errorValue := databaseConnection.Model(&Event{}).
		Joins(eventLaneJoin).
		Joins(eventCalendarJoin).
		Where("calendars.organizer_id = ? AND events.venue_id IS NOT NULL", ownerUserID).
		Distinct("events.venue_id").
		Pluck("events.venue_id", &identifiers).Error
	return identifiers, errorValue
}

// RecalculateFiniteLaneEnd sets one active finite lane end to its last event boundary.
func RecalculateFiniteLaneEnd(databaseConnection *gorm.DB, laneID string) error {
	var lane Lane
	if findError := databaseConnection.First(&lane, "id = ?", laneID).Error; findError != nil {
		return findError
	}
	if lane.Status != LaneStatusActive || lane.EndsAt == nil {
		return nil
	}
	var eventSeriesCount int64
	if countError := databaseConnection.Model(&EventSeries{}).Where("lane_id = ?", laneID).Count(&eventSeriesCount).Error; countError != nil {
		return fmt.Errorf("count event series for lane %s: %w", laneID, countError)
	}
	if eventSeriesCount != 0 {
		return nil
	}
	var laneEvents []Event
	if findError := databaseConnection.Where("lane_id = ?", laneID).Find(&laneEvents).Error; findError != nil {
		return findError
	}
	if len(laneEvents) == 0 {
		return nil
	}
	var lastBoundary time.Time
	for eventIndex := range laneEvents {
		_, eventEnd, boundsError := laneEvents[eventIndex].MarkerBounds()
		if boundsError != nil {
			return boundsError
		}
		if eventEnd.After(lastBoundary) {
			lastBoundary = eventEnd
		}
	}
	lane.EndsAt = &lastBoundary
	return databaseConnection.Save(&lane).Error
}

// CreateLocalIntervalEvent creates one independent or dependent local event with canonical lane membership.
func CreateLocalIntervalEvent(databaseConnection *gorm.DB, organizer *User, calendarID string, anchorEventID *string, title string, description string, venueID *string, startsAt time.Time, endsAt time.Time, referenceTime time.Time, timezone Timezone) (*Event, error) {
	if confirmationError := organizer.ConfirmTimezone(databaseConnection, timezone); confirmationError != nil {
		return nil, confirmationError
	}
	var lane Lane
	relation := IndependentEventRelation()
	if anchorEventID == nil {
		var calendar *Calendar
		if calendarID == "" {
			defaultCalendar, calendarError := EnsureDefaultCalendar(databaseConnection, organizer.ID)
			if calendarError != nil {
				return nil, calendarError
			}
			calendar = defaultCalendar
		} else {
			var selectedCalendar Calendar
			if findError := databaseConnection.Where("id = ? AND organizer_id = ?", calendarID, organizer.ID).First(&selectedCalendar).Error; findError != nil {
				return nil, fmt.Errorf("find calendar %s for organizer %s: %w", calendarID, organizer.ID, findError)
			}
			calendar = &selectedCalendar
		}
		displayOrder, orderError := NextLaneDisplayOrder(databaseConnection, calendar.ID)
		if orderError != nil {
			return nil, orderError
		}
		newLane, laneError := NewFiniteLane(calendar.ID, title, referenceTime, endsAt, displayOrder)
		if laneError != nil {
			return nil, laneError
		}
		if createError := databaseConnection.Create(newLane).Error; createError != nil {
			return nil, fmt.Errorf("create independent event lane: %w", createError)
		}
		lane = *newLane
	} else {
		var anchor Event
		if findError := anchor.FindByIDAndOwner(databaseConnection, *anchorEventID, organizer.ID); findError != nil {
			return nil, fmt.Errorf("find anchor event %s: %w", *anchorEventID, findError)
		}
		if anchor.RelationType == EventRelationDependent || (calendarID != "" && anchor.Lane.CalendarID != calendarID) {
			return nil, ErrEventMembershipInvalid
		}
		lane = anchor.Lane
		dependentRelation, relationError := DependentEventRelation(anchor.ID)
		if relationError != nil {
			return nil, relationError
		}
		relation = dependentRelation
		if lane.EndsAt != nil && endsAt.After(*lane.EndsAt) {
			canonicalEnd := endsAt.UTC()
			lane.EndsAt = &canonicalEnd
			if updateError := databaseConnection.Save(&lane).Error; updateError != nil {
				return nil, fmt.Errorf("extend dependency lane %s: %w", lane.ID, updateError)
			}
		}
	}
	eventTime, eventTimeError := NewIntervalEventTime(startsAt, endsAt, timezone)
	if eventTimeError != nil {
		return nil, eventTimeError
	}
	event, eventError := NewEvent(lane.ID, title, description, venueID, relation, eventTime)
	if eventError != nil {
		return nil, eventError
	}
	if createError := event.Create(databaseConnection); createError != nil {
		return nil, createError
	}
	if boundsError := RecalculateFiniteLaneEnd(databaseConnection, lane.ID); boundsError != nil {
		return nil, fmt.Errorf("recalculate lane %s after event creation: %w", lane.ID, boundsError)
	}
	return event, nil
}

// CreateSeriesOccurrenceIntervalEvent creates one local occurrence on its event series lane.
func CreateSeriesOccurrenceIntervalEvent(databaseConnection *gorm.DB, organizerID string, eventSeriesID string, title string, description string, venueID *string, startsAt time.Time, endsAt time.Time, timezone Timezone) (*Event, error) {
	var createdEvent *Event
	transactionError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
		var createError error
		createdEvent, createError = createSeriesOccurrenceIntervalEvent(transaction, organizerID, eventSeriesID, title, description, venueID, startsAt, endsAt, timezone)
		return createError
	})
	return createdEvent, transactionError
}

func createSeriesOccurrenceIntervalEvent(databaseConnection *gorm.DB, organizerID string, eventSeriesID string, title string, description string, venueID *string, startsAt time.Time, endsAt time.Time, timezone Timezone) (*Event, error) {
	var series EventSeries
	findError := databaseConnection.Model(&EventSeries{}).
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableEventSeries+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableEventSeries+".id = ? AND "+config.TableCalendars+".organizer_id = ?", eventSeriesID, organizerID).
		First(&series).Error
	if findError != nil {
		return nil, fmt.Errorf("find event series %s for organizer %s: %w", eventSeriesID, organizerID, findError)
	}
	var lane Lane
	if laneError := databaseConnection.First(&lane, "id = ?", series.LaneID).Error; laneError != nil {
		return nil, fmt.Errorf("find event series lane %s: %w", series.LaneID, laneError)
	}
	relation, relationError := SeriesOccurrenceRelation(series.ID)
	if relationError != nil {
		return nil, relationError
	}
	eventTime, eventTimeError := NewIntervalEventTime(startsAt, endsAt, timezone)
	if eventTimeError != nil {
		return nil, eventTimeError
	}
	event, eventError := NewEvent(lane.ID, title, description, venueID, relation, eventTime)
	if eventError != nil {
		return nil, eventError
	}
	if createError := event.Create(databaseConnection); createError != nil {
		return nil, createError
	}
	return event, nil
}
