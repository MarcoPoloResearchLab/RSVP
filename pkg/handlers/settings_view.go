package handlers

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// SettingsViewData contains the organizer-owned resources shown in the global settings dialog.
type SettingsViewData struct {
	Calendars                []SettingsCalendarView
	CalendarCreateURL        string
	LaneCreateURL            string
	CalendarAuthorizationURL string
	CalendarConnection       *SettingsConnectionView
}

// SettingsCalendarView contains one calendar's editable presentation fields.
type SettingsCalendarView struct {
	ID            string
	Name          string
	Symbol        string
	ColorToken    string
	ManagementURL string
	PreviousOrder int
	NextOrder     int
	CanMoveUp     bool
	CanMoveDown   bool
}

// SettingsConnectionView reports one Google Calendar connection and its automatic synchronization state.
type SettingsConnectionView struct {
	ID                 string
	Status             models.CalendarConnectionStatus
	SyncState          models.CalendarSyncState
	SyncError          bool
	LastSuccessfulSync string
	ManagementURL      string
	SourceCalendarURL  string
}

func newSettingsViewData(database *gorm.DB, organizerID string) (SettingsViewData, error) {
	calendars, calendarsError := models.FindCalendarsByOwner(database, organizerID)
	if calendarsError != nil {
		return SettingsViewData{}, fmt.Errorf("find settings calendars for organizer %s: %w", organizerID, calendarsError)
	}
	viewData := SettingsViewData{
		Calendars:                make([]SettingsCalendarView, 0, len(calendars)),
		CalendarCreateURL:        config.WebCalendars,
		LaneCreateURL:            config.WebLanes,
		CalendarAuthorizationURL: config.WebCalendarAuthorizationRequests,
	}
	for calendarIndex, calendar := range calendars {
		viewData.Calendars = append(viewData.Calendars, SettingsCalendarView{
			ID: calendar.ID, Name: calendar.Name, Symbol: calendar.Symbol, ColorToken: calendar.ColorToken,
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
		ManagementURL:     config.WebCalendarConnections + url.PathEscape(connection.ID),
		SourceCalendarURL: config.WebCalendarConnections + url.PathEscape(connection.ID) + "/source-calendars/",
	}
	if syncStateError := loadSettingsConnectionSyncState(database, connection.ID, viewData.CalendarConnection); syncStateError != nil {
		return SettingsViewData{}, syncStateError
	}
	return viewData, nil
}

func loadSettingsConnectionSyncState(database *gorm.DB, connectionID string, connectionView *SettingsConnectionView) error {
	var synchronizations []models.CalendarSync
	queryError := database.Model(&models.CalendarSync{}).
		Joins("JOIN source_calendar_mappings ON source_calendar_mappings.id = calendar_syncs.mapping_id AND source_calendar_mappings.deleted_at IS NULL").
		Where("source_calendar_mappings.connection_id = ?", connectionID).
		Order("calendar_syncs.started_at DESC").
		Find(&synchronizations).Error
	if queryError != nil {
		return fmt.Errorf("read synchronization state for calendar connection %s: %w", connectionID, queryError)
	}
	seenMappings := make(map[string]bool)
	for synchronizationIndex := range synchronizations {
		synchronization := &synchronizations[synchronizationIndex]
		if synchronization.State == models.CalendarSyncSucceeded && synchronization.FinishedAt != nil {
			formattedTime := synchronization.FinishedAt.UTC().Format(time.RFC3339)
			if connectionView.LastSuccessfulSync == "" || formattedTime > connectionView.LastSuccessfulSync {
				connectionView.LastSuccessfulSync = formattedTime
			}
		}
		if seenMappings[synchronization.MappingID] {
			continue
		}
		seenMappings[synchronization.MappingID] = true
		switch synchronization.State {
		case models.CalendarSyncFailed:
			connectionView.SyncState = models.CalendarSyncFailed
			connectionView.SyncError = true
		case models.CalendarSyncRunning:
			if !connectionView.SyncError {
				connectionView.SyncState = models.CalendarSyncRunning
			}
		case models.CalendarSyncPending:
			if connectionView.SyncState == "" || connectionView.SyncState == models.CalendarSyncSucceeded {
				connectionView.SyncState = models.CalendarSyncPending
			}
		case models.CalendarSyncSucceeded:
			if connectionView.SyncState == "" {
				connectionView.SyncState = models.CalendarSyncSucceeded
			}
		}
	}
	return nil
}
