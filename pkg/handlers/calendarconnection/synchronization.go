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
	MappingID          string
	State              models.CalendarSyncState
	LastSuccessfulSync string
}

func readConnectionSynchronization(database *gorm.DB, connectionID string) (connectionSynchronization, error) {
	rankedSynchronizations := database.Model(&models.CalendarSync{}).
		Select(
			"calendar_syncs.mapping_id, calendar_syncs.state, "+
				"ROW_NUMBER() OVER (PARTITION BY calendar_syncs.mapping_id ORDER BY calendar_syncs.started_at DESC, calendar_syncs.created_at DESC, calendar_syncs.id DESC) AS synchronization_rank, "+
				"COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', MAX(CASE WHEN calendar_syncs.state = ? THEN calendar_syncs.finished_at END) OVER ()), '') AS last_successful_sync",
			models.CalendarSyncSucceeded,
		).
		Joins("JOIN source_calendar_mappings ON source_calendar_mappings.id = calendar_syncs.mapping_id AND source_calendar_mappings.deleted_at IS NULL").
		Where("source_calendar_mappings.connection_id = ?", connectionID)

	var rows []connectionSynchronizationRow
	queryError := database.Table("(?) AS ranked_synchronizations", rankedSynchronizations).
		Select("mapping_id, state, last_successful_sync").
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
