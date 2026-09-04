package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

const (
	// DefaultHorizonWindowDays is the default JSON projection day count.
	DefaultHorizonWindowDays = 90
	// MaximumHorizonWindowDays is the largest accepted local calendar-day count.
	MaximumHorizonWindowDays = 366
	allDayQueryMargin        = 24 * time.Hour
	probeMarkerTitle         = "Probe"
)

// HorizonMarkerType identifies one closed projection marker variant.
type HorizonMarkerType string

const (
	// HorizonMarkerEvent identifies an event marker.
	HorizonMarkerEvent HorizonMarkerType = "event"
	// HorizonMarkerProbe identifies a probe marker.
	HorizonMarkerProbe HorizonMarkerType = "probe"
	// HorizonMarkerDerived identifies a calculated derived marker.
	HorizonMarkerDerived HorizonMarkerType = "derived"
)

var (
	// ErrInvalidHorizonWindow indicates that a requested projection window is invalid.
	ErrInvalidHorizonWindow = errors.New("invalid horizon window")
)

// HorizonWindow is one valid half-open projection window.
type HorizonWindow struct {
	start    time.Time
	end      time.Time
	timezone models.Timezone
}

// NewDefaultHorizonWindow constructs the default JSON projection window.
func NewDefaultHorizonWindow(referenceTime time.Time, timezone models.Timezone) (HorizonWindow, error) {
	if referenceTime.IsZero() {
		return HorizonWindow{}, ErrInvalidHorizonWindow
	}
	location, locationError := timezone.Location()
	if locationError != nil {
		return HorizonWindow{}, fmt.Errorf("load default horizon timezone: %w", locationError)
	}
	localReference := referenceTime.In(location)
	localStart := time.Date(localReference.Year(), localReference.Month(), localReference.Day(), 0, 0, 0, 0, location)
	return newHorizonWindow(localStart, localStart.AddDate(0, 0, DefaultHorizonWindowDays), timezone)
}

// NewHorizonWindow constructs one explicit half-open projection window.
func NewHorizonWindow(start time.Time, end time.Time, timezone models.Timezone) (HorizonWindow, error) {
	return newHorizonWindow(start, end, timezone)
}

func newHorizonWindow(start time.Time, end time.Time, timezone models.Timezone) (HorizonWindow, error) {
	window := HorizonWindow{start: start.UTC(), end: end.UTC(), timezone: timezone}
	if validationError := window.validate(); validationError != nil {
		return HorizonWindow{}, validationError
	}
	return window, nil
}

func (window HorizonWindow) validate() error {
	location, locationError := window.timezone.Location()
	if locationError != nil {
		return fmt.Errorf("load horizon timezone: %w", locationError)
	}
	if window.start.IsZero() || window.end.IsZero() || !window.end.After(window.start) {
		return ErrInvalidHorizonWindow
	}
	maximumEnd := window.start.In(location).AddDate(0, 0, MaximumHorizonWindowDays)
	if window.end.After(maximumEnd) {
		return ErrInvalidHorizonWindow
	}
	return nil
}

// HorizonWindowProjection is the serialized time window.
type HorizonWindowProjection struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// HorizonProjection contains all owner calendars and visible lane data for one window.
type HorizonProjection struct {
	Window    HorizonWindowProjection     `json:"window"`
	Calendars []HorizonCalendarProjection `json:"calendars"`
}

// HorizonCalendarProjection contains one calendar and its intersecting lanes.
type HorizonCalendarProjection struct {
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	ColorToken     string                  `json:"color_token"`
	DisplayOrder   int                     `json:"display_order"`
	Visible        bool                    `json:"visible"`
	Lanes          []HorizonLaneProjection `json:"lanes"`
	TotalLaneCount int                     `json:"-"`
}

// HorizonLaneProjection contains one lane and its markers in the window.
type HorizonLaneProjection struct {
	ID           string                      `json:"id"`
	Title        string                      `json:"title"`
	Status       models.LaneStatus           `json:"status"`
	StartsAt     string                      `json:"starts_at"`
	EndsAt       *string                     `json:"ends_at"`
	DisplayOrder int                         `json:"display_order"`
	Attention    *HorizonAttentionProjection `json:"attention_policy,omitempty"`
	Markers      []HorizonMarkerProjection   `json:"markers"`
}

// HorizonAttentionProjection contains one lane attention policy.
type HorizonAttentionProjection struct {
	ID                        string `json:"id"`
	ReviewIntervalSeconds     int64  `json:"review_interval_seconds"`
	NextProbeAt               string `json:"next_probe_at"`
	EscalationIntervalSeconds *int64 `json:"escalation_interval_seconds"`
}

// HorizonMarkerTimeProjection is one closed marker time shape.
type HorizonMarkerTimeProjection struct {
	Shape     models.EventTimeShape `json:"shape"`
	At        string                `json:"at,omitempty"`
	Start     string                `json:"start,omitempty"`
	End       string                `json:"end,omitempty"`
	StartDate string                `json:"start_date,omitempty"`
	EndDate   string                `json:"end_date,omitempty"`
	Timezone  string                `json:"timezone"`
}

// HorizonMarkerProjection is one typed event or probe marker.
type HorizonMarkerProjection struct {
	ID             string                      `json:"id"`
	Type           HorizonMarkerType           `json:"type"`
	Title          string                      `json:"title"`
	LaneID         string                      `json:"lane_id"`
	Time           HorizonMarkerTimeProjection `json:"time"`
	EventID        string                      `json:"event_id,omitempty"`
	RelationType   models.EventRelationType    `json:"relation_type,omitempty"`
	ProbeID        string                      `json:"probe_id,omitempty"`
	DueAt          string                      `json:"due_at,omitempty"`
	ProbeState     models.ProbeState           `json:"probe_state,omitempty"`
	RuleID         string                      `json:"rule_id,omitempty"`
	AnchorMarkerID string                      `json:"anchor_marker_id,omitempty"`
}

type horizonLanePosition struct {
	calendarIndex int
	laneIndex     int
}

// HorizonProjectionService reads the complete owner-scoped horizon projection.
type HorizonProjectionService struct {
	database *gorm.DB
}

// NewHorizonProjectionService constructs the projection service.
func NewHorizonProjectionService(database *gorm.DB) (*HorizonProjectionService, error) {
	if database == nil {
		return nil, errors.New("horizon database is required")
	}
	return &HorizonProjectionService{database: database}, nil
}

// Project returns one horizon projection for an organizer.
func (service *HorizonProjectionService) Project(ctx context.Context, organizerID string, window HorizonWindow) (HorizonProjection, error) {
	if organizerID == "" {
		return HorizonProjection{}, models.ErrOrganizerIDRequired
	}
	if validationError := window.validate(); validationError != nil {
		return HorizonProjection{}, validationError
	}
	var projection HorizonProjection
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		transactionService := &HorizonProjectionService{database: transaction}
		var projectionError error
		projection, projectionError = transactionService.project(ctx, organizerID, window)
		return projectionError
	}, &sql.TxOptions{ReadOnly: true})
	if transactionError != nil {
		return HorizonProjection{}, fmt.Errorf("project horizon for organizer %s: %w", organizerID, transactionError)
	}
	return projection, nil
}

func (service *HorizonProjectionService) project(ctx context.Context, organizerID string, window HorizonWindow) (HorizonProjection, error) {
	projection := HorizonProjection{
		Window: HorizonWindowProjection{
			Start:    formatHorizonTime(window.start),
			End:      formatHorizonTime(window.end),
			Timezone: window.timezone.String(),
		},
		Calendars: make([]HorizonCalendarProjection, 0),
	}

	var calendars []models.Calendar
	if queryError := service.database.WithContext(ctx).
		Where("organizer_id = ?", organizerID).
		Order("display_order ASC").
		Order("id ASC").
		Find(&calendars).Error; queryError != nil {
		return HorizonProjection{}, fmt.Errorf("read horizon calendars for organizer %s: %w", organizerID, queryError)
	}

	calendarPositions := make(map[string]int, len(calendars))
	calendarIDs := make([]string, 0, len(calendars))
	for _, calendar := range calendars {
		calendarIDs = append(calendarIDs, calendar.ID)
	}
	laneCounts := make(map[string]int, len(calendarIDs))
	if len(calendarIDs) != 0 {
		var rows []struct {
			CalendarID string
			LaneCount  int
		}
		if countError := service.database.WithContext(ctx).Model(&models.Lane{}).
			Select("calendar_id, COUNT(*) AS lane_count").Where("calendar_id IN ?", calendarIDs).
			Group("calendar_id").Scan(&rows).Error; countError != nil {
			return HorizonProjection{}, fmt.Errorf("count horizon lanes for organizer %s: %w", organizerID, countError)
		}
		for _, row := range rows {
			laneCounts[row.CalendarID] = row.LaneCount
		}
	}
	for _, calendar := range calendars {
		calendarPositions[calendar.ID] = len(projection.Calendars)
		projection.Calendars = append(projection.Calendars, HorizonCalendarProjection{
			ID: calendar.ID, Name: calendar.Name, ColorToken: calendar.ColorToken,
			DisplayOrder: calendar.DisplayOrder, Visible: calendar.Visible, Lanes: make([]HorizonLaneProjection, 0), TotalLaneCount: laneCounts[calendar.ID],
		})
	}

	var lanes []models.Lane
	laneQuery := service.database.WithContext(ctx).Model(&models.Lane{}).
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableCalendars+".organizer_id = ?", organizerID).
		Where("(("+config.TableLanes+".ends_at IS NOT NULL AND "+config.TableLanes+".starts_at < ? AND "+config.TableLanes+".ends_at >= ?) OR ("+
			config.TableLanes+".status = ? AND "+config.TableLanes+".ends_at IS NULL AND "+config.TableLanes+".starts_at < ?))",
			window.end, window.start, models.LaneStatusActive, window.end).
		Order(config.TableCalendars + ".display_order ASC").
		Order(config.TableLanes + ".display_order ASC").
		Order(config.TableLanes + ".id ASC")
	if queryError := laneQuery.Find(&lanes).Error; queryError != nil {
		return HorizonProjection{}, fmt.Errorf("read horizon lanes for organizer %s: %w", organizerID, queryError)
	}

	lanePositions := make(map[string]horizonLanePosition, len(lanes))
	for _, lane := range lanes {
		calendarIndex, calendarFound := calendarPositions[lane.CalendarID]
		if !calendarFound {
			return HorizonProjection{}, fmt.Errorf("project lane %s: calendar %s is absent", lane.ID, lane.CalendarID)
		}
		var endValue *string
		if lane.EndsAt != nil {
			formattedEnd := formatHorizonTime(*lane.EndsAt)
			endValue = &formattedEnd
		}
		laneIndex := len(projection.Calendars[calendarIndex].Lanes)
		projection.Calendars[calendarIndex].Lanes = append(projection.Calendars[calendarIndex].Lanes, HorizonLaneProjection{
			ID: lane.ID, Title: lane.Title, Status: lane.Status, StartsAt: formatHorizonTime(lane.StartsAt),
			EndsAt: endValue, DisplayOrder: lane.DisplayOrder, Markers: make([]HorizonMarkerProjection, 0),
		})
		lanePositions[lane.ID] = horizonLanePosition{calendarIndex: calendarIndex, laneIndex: laneIndex}
	}

	if len(lanePositions) == 0 {
		return projection, nil
	}
	laneIDs := make([]string, 0, len(lanePositions))
	for laneID := range lanePositions {
		laneIDs = append(laneIDs, laneID)
	}

	if attentionError := service.addAttentionPolicies(ctx, organizerID, laneIDs, lanePositions, &projection); attentionError != nil {
		return HorizonProjection{}, attentionError
	}
	if eventError := service.addEventMarkers(ctx, organizerID, window, laneIDs, lanePositions, &projection); eventError != nil {
		return HorizonProjection{}, eventError
	}
	if probeError := service.addProbeMarkers(ctx, organizerID, window, laneIDs, lanePositions, &projection); probeError != nil {
		return HorizonProjection{}, probeError
	}
	if derivedError := service.addDerivedMarkers(ctx, organizerID, window, laneIDs, lanePositions, &projection); derivedError != nil {
		return HorizonProjection{}, derivedError
	}
	return projection, nil
}

func (service *HorizonProjectionService) addDerivedMarkers(ctx context.Context, organizerID string, window HorizonWindow, laneIDs []string, lanePositions map[string]horizonLanePosition, projection *HorizonProjection) error {
	var markers []models.DerivedMarker
	queryError := service.database.WithContext(ctx).Model(&models.DerivedMarker{}).Preload("Rule").
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableDerivedMarkers+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableCalendars+".organizer_id = ?", organizerID).Where(config.TableDerivedMarkers+".lane_id IN ?", laneIDs).
		Where(config.TableDerivedMarkers+".at >= ? AND "+config.TableDerivedMarkers+".at < ?", window.start, window.end).
		Order(config.TableDerivedMarkers + ".at ASC").Find(&markers).Error
	if queryError != nil {
		return fmt.Errorf("read horizon derived markers for organizer %s: %w", organizerID, queryError)
	}
	for markerIndex := range markers {
		marker := &markers[markerIndex]
		position := lanePositions[marker.LaneID]
		lane := &projection.Calendars[position.calendarIndex].Lanes[position.laneIndex]
		lane.Markers = append(lane.Markers, HorizonMarkerProjection{ID: marker.ID, Type: HorizonMarkerDerived, Title: "Derived marker", LaneID: marker.LaneID, Time: HorizonMarkerTimeProjection{Shape: models.EventTimePoint, At: formatHorizonTime(marker.At), Timezone: marker.Timezone}, RuleID: marker.RuleID, AnchorMarkerID: marker.Rule.AnchorID})
	}
	return nil
}

func (service *HorizonProjectionService) addAttentionPolicies(
	ctx context.Context,
	organizerID string,
	laneIDs []string,
	lanePositions map[string]horizonLanePosition,
	projection *HorizonProjection,
) error {
	var policies []models.AttentionPolicy
	queryError := service.database.WithContext(ctx).Model(&models.AttentionPolicy{}).
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableAttentionPolicies+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableCalendars+".organizer_id = ?", organizerID).
		Where(config.TableAttentionPolicies+".lane_id IN ?", laneIDs).
		Find(&policies).Error
	if queryError != nil {
		return fmt.Errorf("read horizon attention policies for organizer %s: %w", organizerID, queryError)
	}
	for policyIndex := range policies {
		policy := &policies[policyIndex]
		position := lanePositions[policy.LaneID]
		lane := &projection.Calendars[position.calendarIndex].Lanes[position.laneIndex]
		lane.Attention = &HorizonAttentionProjection{
			ID: policy.ID, ReviewIntervalSeconds: policy.ReviewIntervalSeconds,
			NextProbeAt: formatHorizonTime(policy.NextProbeAt), EscalationIntervalSeconds: policy.EscalationIntervalSeconds,
		}
	}
	return nil
}

func (service *HorizonProjectionService) addEventMarkers(
	ctx context.Context,
	organizerID string,
	window HorizonWindow,
	laneIDs []string,
	lanePositions map[string]horizonLanePosition,
	projection *HorizonProjection,
) error {
	var events []models.Event
	coarseAllDayStart := window.start.Add(-allDayQueryMargin).Format(time.DateOnly)
	coarseAllDayEnd := window.end.Add(allDayQueryMargin).Format(time.DateOnly)
	queryError := service.database.WithContext(ctx).Model(&models.Event{}).
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableEvents+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableCalendars+".organizer_id = ?", organizerID).
		Where(config.TableEvents+".lane_id IN ?", laneIDs).
		Where("(("+config.TableEvents+".time_shape = ? AND "+config.TableEvents+".at >= ? AND "+config.TableEvents+".at < ?) OR ("+
			config.TableEvents+".time_shape = ? AND "+config.TableEvents+".starts_at < ? AND "+config.TableEvents+".ends_at > ?) OR ("+
			config.TableEvents+".time_shape = ? AND "+config.TableEvents+".start_date < ? AND "+config.TableEvents+".end_date > ?))",
			models.EventTimePoint, window.start, window.end,
			models.EventTimeInterval, window.end, window.start,
			models.EventTimeAllDay, coarseAllDayEnd, coarseAllDayStart).
		Order(config.TableEvents + ".id ASC").
		Find(&events).Error
	if queryError != nil {
		return fmt.Errorf("read horizon events for organizer %s: %w", organizerID, queryError)
	}
	for eventIndex := range events {
		event := &events[eventIndex]
		included, boundsError := eventIntersectsHorizonWindow(event, window)
		if boundsError != nil {
			return fmt.Errorf("read event marker %s bounds: %w", event.ID, boundsError)
		}
		if !included {
			continue
		}
		position := lanePositions[event.LaneID]
		lane := &projection.Calendars[position.calendarIndex].Lanes[position.laneIndex]
		lane.Markers = append(lane.Markers, eventMarkerProjection(event))
	}
	return nil
}

func (service *HorizonProjectionService) addProbeMarkers(
	ctx context.Context,
	organizerID string,
	window HorizonWindow,
	laneIDs []string,
	lanePositions map[string]horizonLanePosition,
	projection *HorizonProjection,
) error {
	var probes []models.Probe
	queryError := service.database.WithContext(ctx).Model(&models.Probe{}).
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableProbes+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableCalendars+".organizer_id = ?", organizerID).
		Where(config.TableProbes+".lane_id IN ?", laneIDs).
		Where(config.TableProbes+".due_at >= ? AND "+config.TableProbes+".due_at < ?", window.start, window.end).
		Order(config.TableProbes + ".due_at ASC").
		Order(config.TableProbes + ".id ASC").
		Find(&probes).Error
	if queryError != nil {
		return fmt.Errorf("read horizon probes for organizer %s: %w", organizerID, queryError)
	}
	for probeIndex := range probes {
		probe := &probes[probeIndex]
		position := lanePositions[probe.LaneID]
		lane := &projection.Calendars[position.calendarIndex].Lanes[position.laneIndex]
		dueAt := formatHorizonTime(probe.DueAt)
		lane.Markers = append(lane.Markers, HorizonMarkerProjection{
			ID: probe.ID, Type: HorizonMarkerProbe, Title: probeMarkerTitle, LaneID: probe.LaneID,
			Time:    HorizonMarkerTimeProjection{Shape: models.EventTimePoint, At: dueAt, Timezone: window.timezone.String()},
			ProbeID: probe.ID, DueAt: dueAt, ProbeState: probe.State,
		})
	}
	return nil
}

func eventIntersectsHorizonWindow(event *models.Event, window HorizonWindow) (bool, error) {
	markerStart, markerEnd, boundsError := event.MarkerBounds()
	if boundsError != nil {
		return false, boundsError
	}
	if event.TimeShape == models.EventTimePoint {
		return !markerStart.Before(window.start) && markerStart.Before(window.end), nil
	}
	return markerStart.Before(window.end) && markerEnd.After(window.start), nil
}

func eventMarkerProjection(event *models.Event) HorizonMarkerProjection {
	markerTime := HorizonMarkerTimeProjection{Shape: event.TimeShape, Timezone: event.Timezone}
	switch event.TimeShape {
	case models.EventTimePoint:
		markerTime.At = formatHorizonTime(*event.At)
	case models.EventTimeInterval:
		markerTime.Start = formatHorizonTime(*event.StartsAt)
		markerTime.End = formatHorizonTime(*event.EndsAt)
	case models.EventTimeAllDay:
		markerTime.StartDate = *event.StartDate
		markerTime.EndDate = *event.EndDate
	}
	return HorizonMarkerProjection{
		ID: event.ID, Type: HorizonMarkerEvent, Title: event.Title, LaneID: event.LaneID, Time: markerTime,
		EventID: event.ID, RelationType: event.RelationType,
	}
}

func formatHorizonTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
