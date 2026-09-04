package handlers

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// SettingsViewData contains the organizer-owned resources shown in the global settings dialog.
type SettingsViewData struct {
	Calendars                []SettingsCalendarView
	CalendarCreateURL        string
	LaneCreateURL            string
	OrganizerManagementURL   string
	OrganizerTimezone        string
	CalendarAuthorizationURL string
	CalendarConnection       *SettingsConnectionView
}

// SettingsCalendarView contains one calendar's editable presentation fields.
type SettingsCalendarView struct {
	ID            string
	Name          string
	ColorToken    string
	ManagementURL string
	PreviousOrder int
	NextOrder     int
	CanMoveUp     bool
	CanMoveDown   bool
}

// SettingsConnectionView contains one Google Calendar connection's management links.
type SettingsConnectionView struct {
	ID            string
	Status        models.CalendarConnectionStatus
	ManagementURL string
}

func newSettingsViewData(database *gorm.DB, organizerID string) (SettingsViewData, error) {
	var organizer models.User
	if organizerError := organizer.FindByID(database, organizerID); organizerError != nil {
		return SettingsViewData{}, fmt.Errorf("find settings organizer %s: %w", organizerID, organizerError)
	}
	calendars, calendarsError := models.FindCalendarsByOwner(database, organizerID)
	if calendarsError != nil {
		return SettingsViewData{}, fmt.Errorf("find settings calendars for organizer %s: %w", organizerID, calendarsError)
	}
	viewData := SettingsViewData{
		Calendars:                make([]SettingsCalendarView, 0, len(calendars)),
		CalendarCreateURL:        config.WebCalendars,
		LaneCreateURL:            config.WebLanes,
		OrganizerManagementURL:   config.WebOrganizers + url.PathEscape(organizerID),
		CalendarAuthorizationURL: config.WebCalendarAuthorizationRequests,
	}
	if organizer.Timezone != nil {
		viewData.OrganizerTimezone = *organizer.Timezone
	}
	for calendarIndex, calendar := range calendars {
		viewData.Calendars = append(viewData.Calendars, SettingsCalendarView{
			ID: calendar.ID, Name: calendar.Name, ColorToken: calendar.ColorToken,
			ManagementURL: config.WebCalendars + url.PathEscape(calendar.ID), PreviousOrder: calendar.DisplayOrder - 1,
			NextOrder: calendar.DisplayOrder + 1, CanMoveUp: calendarIndex > 0, CanMoveDown: calendarIndex < len(calendars)-1,
		})
	}

	var connection models.CalendarConnection
	connectionError := database.First(&connection, "organizer_id = ?", organizerID).Error
	if errors.Is(connectionError, gorm.ErrRecordNotFound) {
		return viewData, nil
	}
	if connectionError != nil {
		return SettingsViewData{}, fmt.Errorf("find settings calendar connection for organizer %s: %w", organizerID, connectionError)
	}
	viewData.CalendarConnection = &SettingsConnectionView{
		ID: connection.ID, Status: connection.Status,
		ManagementURL: config.WebCalendarConnections + url.PathEscape(connection.ID),
	}
	return viewData, nil
}
