package horizon

import (
	"fmt"
	"html/template"
	"net/url"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/services"
)

type horizonViewData struct {
	NeedsTimezoneSetup      bool
	Window                  services.HorizonWindowProjection
	StylesURL               string
	ScriptURL               string
	Scale                   string
	BackwardWindowURL       string
	ForwardWindowURL        string
	DayScaleURL             string
	WeekScaleURL            string
	MonthScaleURL           string
	YearScaleURL            string
	TodayStyle              template.HTMLAttr
	TimeScaleTicks          []horizonTimeScaleTick
	Calendars               []horizonCalendarView
	CalendarCreateURL       string
	LaneCreateURL           string
	AttentionCreateURL      string
	DerivedCreateURL        string
	IngestionDraftCreateURL string
	IngestionDraftViews     []horizonDraftView
	AnchorEvents            []horizonAnchorView
}

func newHorizonSetupViewData() horizonViewData {
	return horizonViewData{
		NeedsTimezoneSetup: true,
		StylesURL:          config.HorizonStylesPath,
		ScriptURL:          config.HorizonScriptPath,
		CalendarCreateURL:  config.WebCalendars,
	}
}

type horizonAnchorView struct {
	ID    string
	Title string
}
type horizonDraftView struct {
	ID                        string
	Status                    models.IngestionDraftStatus
	Mode                      models.IngestionDraftMode
	Source                    models.IngestionDraftSource
	CalendarID                string
	CalendarName              string
	Title                     string
	AnchorEventID             string
	StartsAt                  string
	EndsAt                    string
	ReviewIntervalSeconds     string
	NextProbeAt               string
	EscalationIntervalSeconds string
	ReferenceTime             string
	Timezone                  string
	ManagementURL             string
	ConfirmationURL           string
	ProposedLane              string
	MissingFields             []string
	DerivedRuleSummaries      []string
}

type horizonTimeScaleTick struct {
	DateTime string
	Label    string
	Position string
}

type horizonCalendarView struct {
	ID            string
	Name          string
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
	ID             string
	Type           services.HorizonMarkerType
	Title          string
	Position       string
	IsLaneTerminal bool
	IsEvent        bool
	EventURL       string
	RSVPURL        string
	ProbeState     models.ProbeState
	ProbeURL       string
	CanComplete    bool
	IsDerived      bool
	RuleID         string
	AnchorMarkerID string
	RuleURL        string
}

func newHorizonViewData(projection services.HorizonProjection, referenceTime time.Time, scale horizonScale) (horizonViewData, error) {
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
		Window: projection.Window, Scale: string(scale),
		StylesURL: config.HorizonStylesPath, ScriptURL: config.HorizonScriptPath,
		CalendarCreateURL: config.WebCalendars, LaneCreateURL: config.WebLanes,
		AttentionCreateURL:      config.WebAttentionPolicies,
		DerivedCreateURL:        config.WebDerivedMarkerRules,
		IngestionDraftCreateURL: config.WebIngestionDrafts,
		IngestionDraftViews:     make([]horizonDraftView, 0),
		AnchorEvents:            make([]horizonAnchorView, 0),
		TimeScaleTicks:          make([]horizonTimeScaleTick, 0), Calendars: make([]horizonCalendarView, 0, len(projection.Calendars)),
	}
	localStart := windowStart.In(location)
	viewData.BackwardWindowURL = horizonScaleURL(scale, shiftHorizonScaleStart(windowStart, location, scale, -1))
	viewData.ForwardWindowURL = horizonScaleURL(scale, shiftHorizonScaleStart(windowStart, location, scale, 1))
	viewData.DayScaleURL = horizonScaleURL(horizonScaleDay, localStart)
	viewData.WeekScaleURL = horizonScaleURL(horizonScaleWeek, localStart)
	viewData.MonthScaleURL = horizonScaleURL(horizonScaleMonth, localStart)
	viewData.YearScaleURL = horizonScaleURL(horizonScaleYear, localStart)
	viewData.TimeScaleTicks = horizonScaleTicks(scale, localStart, windowEnd)
	localReference := referenceTime.In(location)
	today := time.Date(localReference.Year(), localReference.Month(), localReference.Day(), 0, 0, 0, 0, location)
	if !today.Before(windowStart) && today.Before(windowEnd) {
		viewData.TodayStyle = template.HTMLAttr(`style="--today-position: ` + horizonPosition(today, windowStart, windowEnd) + `"`)
	}

	for calendarIndex, calendar := range projection.Calendars {
		calendarView := horizonCalendarView{
			ID: calendar.ID, Name: calendar.Name, ColorToken: calendar.ColorToken,
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
			var finalMarkerTime time.Time
			finalMarkerIndex := -1
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
				if finalMarkerIndex == -1 || markerTime.After(finalMarkerTime) {
					finalMarkerTime = markerTime
					finalMarkerIndex = len(laneView.Markers)
				}
				markerView := horizonMarkerView{
					ID: marker.ID, Type: marker.Type, Title: marker.Title,
					Position:  horizonPosition(markerTime, windowStart, windowEnd),
					IsEvent:   marker.Type == services.HorizonMarkerEvent,
					IsDerived: marker.Type == services.HorizonMarkerDerived,
				}
				if markerView.IsDerived {
					markerView.RuleID, markerView.AnchorMarkerID = marker.RuleID, marker.AnchorMarkerID
					markerView.RuleURL = config.WebDerivedMarkerRules + url.PathEscape(marker.RuleID)
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
					viewData.AnchorEvents = append(viewData.AnchorEvents, horizonAnchorView{ID: marker.EventID, Title: marker.Title})
				}
				laneView.Markers = append(laneView.Markers, markerView)
			}
			if endStyle == "is-finite" && finalMarkerIndex >= 0 {
				laneView.EndPosition = horizonPosition(finalMarkerTime, windowStart, windowEnd)
				laneView.EndStyle += " is-marker-terminated"
				laneView.Markers[finalMarkerIndex].IsLaneTerminal = true
			}
			calendarView.Lanes = append(calendarView.Lanes, laneView)
		}
		viewData.Calendars = append(viewData.Calendars, calendarView)
	}
	return viewData, nil
}

func horizonScaleTicks(scale horizonScale, windowStart time.Time, windowEnd time.Time) []horizonTimeScaleTick {
	ticks := make([]horizonTimeScaleTick, 0)
	for tickTime := windowStart; tickTime.Before(windowEnd); tickTime = nextHorizonScaleTick(scale, tickTime) {
		ticks = append(ticks, horizonTimeScaleTick{
			DateTime: tickTime.Format(time.RFC3339),
			Label:    horizonScaleTickLabel(scale, tickTime),
			Position: horizonPosition(tickTime, windowStart, windowEnd),
		})
	}
	return ticks
}

func nextHorizonScaleTick(scale horizonScale, tickTime time.Time) time.Time {
	switch scale {
	case horizonScaleDay:
		return tickTime.Add(3 * time.Hour)
	case horizonScaleWeek:
		return tickTime.AddDate(0, 0, 1)
	case horizonScaleMonth:
		return tickTime.AddDate(0, 0, 7)
	case horizonScaleYear:
		return tickTime.AddDate(0, 1, 0)
	default:
		panic("invalid Horizon scale")
	}
}

func horizonScaleTickLabel(scale horizonScale, tickTime time.Time) string {
	switch scale {
	case horizonScaleDay:
		return tickTime.Format("3 PM")
	case horizonScaleWeek:
		return tickTime.Format("Mon 2")
	case horizonScaleMonth:
		return tickTime.Format("Jan 2")
	case horizonScaleYear:
		return tickTime.Format("Jan")
	default:
		panic("invalid Horizon scale")
	}
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
