package horizon

import (
	"fmt"
	"net/url"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/services"
)

const horizonScaleIntervalDays = 7

type horizonViewData struct {
	Window                   services.HorizonWindowProjection
	StylesURL                string
	ScriptURL                string
	WindowDays               int
	TodayPosition            *string
	TimeScaleTicks           []horizonTimeScaleTick
	Calendars                []horizonCalendarView
	CalendarCreateURL        string
	LaneCreateURL            string
	AttentionCreateURL       string
	CalendarAuthorizationURL string
	CalendarConnection       *horizonConnectionView
}

type horizonConnectionView struct {
	ID                string
	Status            models.CalendarConnectionStatus
	ManagementURL     string
	SourceCalendarURL string
}

type horizonTimeScaleTick struct {
	DateTime string
	Label    string
	Position string
}

type horizonCalendarView struct {
	ID            string
	Name          string
	Symbol        string
	ColorToken    string
	Visible       bool
	VisibilityURL string
	ManagementURL string
	PreviousOrder int
	NextOrder     int
	CanMoveUp     bool
	CanMoveDown   bool
	Lanes         []horizonLaneView
}

type horizonLaneView struct {
	ID            string
	Title         string
	StartsAt      string
	EndsAt        *string
	IsOpen        bool
	EndStyle      string
	StartPosition string
	EndPosition   string
	Markers       []horizonMarkerView
	ManagementURL string
	PreviousOrder int
	NextOrder     int
	CanMoveUp     bool
	CanMoveDown   bool
	CanResolve    bool
	Attention     *horizonAttentionView
}

type horizonAttentionView struct {
	ID                    string
	ReviewIntervalSeconds int64
	NextProbeAt           string
	NextProbeInput        string
	EscalationSeconds     *int64
	ManagementURL         string
}

type horizonMarkerView struct {
	ID          string
	Type        services.HorizonMarkerType
	Title       string
	Position    string
	IsEvent     bool
	EventURL    string
	RSVPURL     string
	ProbeState  models.ProbeState
	ProbeURL    string
	CanComplete bool
}

func newHorizonViewData(projection services.HorizonProjection, referenceTime time.Time) (horizonViewData, error) {
	windowStart, startError := time.Parse(time.RFC3339Nano, projection.Window.Start)
	if startError != nil {
		return horizonViewData{}, fmt.Errorf("parse horizon view start: %w", startError)
	}
	windowEnd, endError := time.Parse(time.RFC3339Nano, projection.Window.End)
	if endError != nil {
		return horizonViewData{}, fmt.Errorf("parse horizon view end: %w", endError)
	}
	location, locationError := time.LoadLocation(projection.Window.Timezone)
	if locationError != nil {
		return horizonViewData{}, fmt.Errorf("load horizon view timezone: %w", locationError)
	}

	viewData := horizonViewData{
		Window:    projection.Window,
		StylesURL: config.HorizonStylesPath, ScriptURL: config.HorizonScriptPath,
		CalendarCreateURL: config.WebCalendars, LaneCreateURL: config.WebLanes,
		AttentionCreateURL:       config.WebAttentionPolicies,
		CalendarAuthorizationURL: config.WebCalendarAuthorizationRequests,
		TimeScaleTicks:           make([]horizonTimeScaleTick, 0), Calendars: make([]horizonCalendarView, 0, len(projection.Calendars)),
	}
	localStart := windowStart.In(location)
	localEnd := windowEnd.In(location)
	for day := localStart; day.Before(localEnd); day = day.AddDate(0, 0, 1) {
		viewData.WindowDays++
	}
	if viewData.WindowDays < 1 {
		viewData.WindowDays = 1
	}

	for tickTime := localStart; tickTime.Before(windowEnd); tickTime = tickTime.AddDate(0, 0, horizonScaleIntervalDays) {
		viewData.TimeScaleTicks = append(viewData.TimeScaleTicks, horizonTimeScaleTick{
			DateTime: tickTime.Format(time.DateOnly),
			Label:    tickTime.Format("Jan 2"),
			Position: horizonPosition(tickTime, windowStart, windowEnd),
		})
	}
	localReference := referenceTime.In(location)
	today := time.Date(localReference.Year(), localReference.Month(), localReference.Day(), 0, 0, 0, 0, location)
	if !today.Before(windowStart) && today.Before(windowEnd) {
		position := horizonPosition(today, windowStart, windowEnd)
		viewData.TodayPosition = &position
	}

	for calendarIndex, calendar := range projection.Calendars {
		calendarView := horizonCalendarView{
			ID: calendar.ID, Name: calendar.Name, Symbol: calendar.Symbol, ColorToken: calendar.ColorToken,
			Visible: calendar.Visible, VisibilityURL: config.WebCalendars + url.PathEscape(calendar.ID),
			ManagementURL: config.WebCalendars + url.PathEscape(calendar.ID), PreviousOrder: calendar.DisplayOrder - 1,
			NextOrder: calendar.DisplayOrder + 1, CanMoveUp: calendarIndex > 0, CanMoveDown: calendarIndex < len(projection.Calendars)-1,
			Lanes: make([]horizonLaneView, 0, len(calendar.Lanes)),
		}
		for _, lane := range calendar.Lanes {
			laneStart, laneStartError := time.Parse(time.RFC3339Nano, lane.StartsAt)
			if laneStartError != nil {
				return horizonViewData{}, fmt.Errorf("parse horizon lane %s start: %w", lane.ID, laneStartError)
			}
			laneEnd := windowEnd
			endStyle := "is-open"
			if lane.EndsAt != nil {
				parsedEnd, laneEndError := time.Parse(time.RFC3339Nano, *lane.EndsAt)
				if laneEndError != nil {
					return horizonViewData{}, fmt.Errorf("parse horizon lane %s end: %w", lane.ID, laneEndError)
				}
				laneEnd = parsedEnd
				endStyle = "is-finite"
				if parsedEnd.After(windowEnd) {
					endStyle = "is-continuing"
				}
			}
			laneView := horizonLaneView{
				ID: lane.ID, Title: lane.Title, StartsAt: lane.StartsAt, EndsAt: lane.EndsAt, IsOpen: lane.EndsAt == nil, EndStyle: endStyle,
				StartPosition: horizonPosition(laneStart, windowStart, windowEnd),
				EndPosition:   horizonPosition(laneEnd, windowStart, windowEnd),
				Markers:       make([]horizonMarkerView, 0, len(lane.Markers)),
				ManagementURL: config.WebLanes + url.PathEscape(lane.ID), PreviousOrder: lane.DisplayOrder - 1,
				NextOrder: lane.DisplayOrder + 1, CanMoveUp: lane.DisplayOrder > 0, CanMoveDown: lane.DisplayOrder < calendar.TotalLaneCount-1,
				CanResolve: lane.Status == models.LaneStatusActive && lane.EndsAt == nil,
			}
			if lane.Attention != nil {
				nextProbeTime, parseError := time.Parse(time.RFC3339Nano, lane.Attention.NextProbeAt)
				if parseError != nil {
					return horizonViewData{}, fmt.Errorf("parse attention policy %s next probe time: %w", lane.Attention.ID, parseError)
				}
				attentionView := &horizonAttentionView{
					ID: lane.Attention.ID, ReviewIntervalSeconds: lane.Attention.ReviewIntervalSeconds,
					NextProbeAt: lane.Attention.NextProbeAt, NextProbeInput: nextProbeTime.In(location).Format("2006-01-02T15:04"),
					EscalationSeconds: lane.Attention.EscalationIntervalSeconds,
					ManagementURL:     config.WebAttentionPolicies + url.PathEscape(lane.Attention.ID),
				}
				laneView.Attention = attentionView
			}
			for _, marker := range lane.Markers {
				markerTime, markerTimeError := horizonMarkerPositionTime(marker.Time)
				if markerTimeError != nil {
					return horizonViewData{}, fmt.Errorf("parse horizon marker %s time: %w", marker.ID, markerTimeError)
				}
				markerView := horizonMarkerView{
					ID: marker.ID, Type: marker.Type, Title: marker.Title,
					Position: horizonPosition(markerTime, windowStart, windowEnd),
					IsEvent:  marker.Type == services.HorizonMarkerEvent,
				}
				if marker.Type == services.HorizonMarkerProbe {
					markerView.ProbeState = marker.ProbeState
					markerView.ProbeURL = config.WebProbes + url.PathEscape(marker.ProbeID)
					markerView.CanComplete = marker.ProbeState == models.ProbeStatePending
				}
				if markerView.IsEvent {
					encodedEventID := url.QueryEscape(marker.EventID)
					markerView.EventURL = config.WebEvents + "?" + config.EventIDParam + "=" + encodedEventID
					markerView.RSVPURL = config.WebRSVPs + "?" + config.EventIDParam + "=" + encodedEventID
				}
				laneView.Markers = append(laneView.Markers, markerView)
			}
			calendarView.Lanes = append(calendarView.Lanes, laneView)
		}
		viewData.Calendars = append(viewData.Calendars, calendarView)
	}
	return viewData, nil
}

func horizonMarkerPositionTime(markerTime services.HorizonMarkerTimeProjection) (time.Time, error) {
	switch markerTime.Shape {
	case models.EventTimePoint:
		return time.Parse(time.RFC3339Nano, markerTime.At)
	case models.EventTimeInterval:
		return time.Parse(time.RFC3339Nano, markerTime.Start)
	case models.EventTimeAllDay:
		location, locationError := time.LoadLocation(markerTime.Timezone)
		if locationError != nil {
			return time.Time{}, locationError
		}
		return time.ParseInLocation(time.DateOnly, markerTime.StartDate, location)
	default:
		return time.Time{}, models.ErrEventTimeInvalid
	}
}

func horizonPosition(value time.Time, windowStart time.Time, windowEnd time.Time) string {
	clippedValue := value
	if clippedValue.Before(windowStart) {
		clippedValue = windowStart
	}
	if clippedValue.After(windowEnd) {
		clippedValue = windowEnd
	}
	position := clippedValue.Sub(windowStart).Seconds() / windowEnd.Sub(windowStart).Seconds() * 100
	return fmt.Sprintf("%.6f", position)
}
