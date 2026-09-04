package calendarconnection

import (
	"fmt"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
)

type connectionSynchronization struct {
	State              models.CalendarSyncState `json:"state"`
	Error              bool                     `json:"error"`
	LastSuccessfulSync string                   `json:"last_successful_sync"`
}

type connectionSynchronizationRow struct {
	SyncStateID        string
	State              models.CalendarSyncState
	LastSuccessfulSync string
}

func readConnectionSynchronization(database *gorm.DB, connectionID string) (connectionSynchronization, error) {
	rankedSynchronizations := database.Model(&models.CalendarSync{}).
		Select(
			"calendar_syncs.sync_state_id, calendar_syncs.state, "+
				"ROW_NUMBER() OVER (PARTITION BY calendar_syncs.sync_state_id ORDER BY calendar_syncs.started_at DESC, calendar_syncs.created_at DESC, calendar_syncs.id DESC) AS synchronization_rank, "+
				"COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', MAX(CASE WHEN calendar_syncs.state = ? THEN calendar_syncs.finished_at END) OVER ()), '') AS last_successful_sync",
			models.CalendarSyncSucceeded,
		).
		Joins("JOIN provider_calendar_sync_states ON provider_calendar_sync_states.id = calendar_syncs.sync_state_id AND provider_calendar_sync_states.deleted_at IS NULL").
		Where("provider_calendar_sync_states.connection_id = ?", connectionID)

	var rows []connectionSynchronizationRow
	queryError := database.Table("(?) AS ranked_synchronizations", rankedSynchronizations).
		Select("sync_state_id, state, last_successful_sync").
		Where("synchronization_rank = ?", 1).
		Scan(&rows).Error
	if queryError != nil {
		return connectionSynchronization{}, fmt.Errorf("read synchronization state for calendar connection %s: %w", connectionID, queryError)
	}

	synchronization := connectionSynchronization{}
	for rowIndex := range rows {
		row := &rows[rowIndex]
		if row.LastSuccessfulSync != "" {
			synchronization.LastSuccessfulSync = row.LastSuccessfulSync
		}
		switch row.State {
		case models.CalendarSyncFailed:
			synchronization.State = models.CalendarSyncFailed
			synchronization.Error = true
		case models.CalendarSyncRunning:
			if !synchronization.Error {
				synchronization.State = models.CalendarSyncRunning
			}
		case models.CalendarSyncPending:
			if synchronization.State == "" || synchronization.State == models.CalendarSyncSucceeded {
				synchronization.State = models.CalendarSyncPending
			}
		case models.CalendarSyncSucceeded:
			if synchronization.State == "" {
				synchronization.State = models.CalendarSyncSucceeded
			}
		}
	}
	return synchronization, nil
}
