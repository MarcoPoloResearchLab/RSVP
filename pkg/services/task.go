package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	utilsscheduler "github.com/tyemirov/utils/scheduler"
	"gorm.io/gorm"
)

const (
	CalendarConnectionTaskInterval   = time.Second
	CalendarConnectionTaskMaxRetries = 5
	calendarConnectionTaskLease      = 5 * time.Minute
	calendarImportFailedCode         = "calendar_import_failed"
)

type taskPayload struct {
	OrganizerID  string
	Kind         models.TaskKind
	ResourceType models.TaskResourceType
	ResourceID   string
}

type taskRepository struct {
	database *gorm.DB
	lease    time.Duration
}

// ReadCalendarConnectionTask returns the import task for one organizer-owned connection.
func ReadCalendarConnectionTask(ctx context.Context, database *gorm.DB, organizerID string, connectionID string) (*models.Task, error) {
	var task models.Task
	findError := database.WithContext(ctx).First(
		&task,
		"organizer_id = ? AND kind = ? AND resource_type = ? AND resource_id = ?",
		organizerID,
		models.TaskKindCalendarConnectionImport,
		models.TaskResourceCalendarConnection,
		connectionID,
	).Error
	if findError != nil {
		return nil, findError
	}
	return &task, nil
}

// IncompleteCalendarConnectionTaskIDs returns connections that the initial import task still owns.
func IncompleteCalendarConnectionTaskIDs(ctx context.Context, database *gorm.DB) (map[string]struct{}, error) {
	var connectionIDs []string
	findError := database.WithContext(ctx).Model(&models.Task{}).
		Where("kind = ? AND resource_type = ? AND state != ?", models.TaskKindCalendarConnectionImport, models.TaskResourceCalendarConnection, models.TaskSucceeded).
		Pluck("resource_id", &connectionIDs).Error
	if findError != nil {
		return nil, fmt.Errorf("read incomplete calendar connection tasks: %w", findError)
	}
	active := make(map[string]struct{}, len(connectionIDs))
	for _, connectionID := range connectionIDs {
		active[connectionID] = struct{}{}
	}
	return active, nil
}

// NewCalendarConnectionTaskWorker constructs the shared scheduler worker for calendar imports.
func NewCalendarConnectionTaskWorker(database *gorm.DB, connectionService *CalendarConnectionService, syncService *CalendarSyncService, applicationLogger *log.Logger) (*utilsscheduler.Worker, error) {
	if database == nil || connectionService == nil || syncService == nil || applicationLogger == nil {
		return nil, errors.New("calendar connection task dependencies are required")
	}
	repository := &taskRepository{database: database, lease: calendarConnectionTaskLease}
	dispatcher := &calendarConnectionTaskDispatcher{connectionService: connectionService, syncService: syncService}
	schedulerLogger := slog.New(slog.NewTextHandler(applicationLogger.Writer(), &slog.HandlerOptions{}))
	return utilsscheduler.NewWorker(utilsscheduler.Config{
		Repository:    repository,
		Dispatcher:    dispatcher,
		Logger:        schedulerLogger,
		Interval:      CalendarConnectionTaskInterval,
		MaxRetries:    CalendarConnectionTaskMaxRetries,
		SuccessStatus: string(models.TaskSucceeded),
		FailureStatus: string(models.TaskFailed),
	})
}

func (repository *taskRepository) PendingJobs(ctx context.Context, maxRetries int, now time.Time) ([]utilsscheduler.Job, error) {
	staleBefore := now.UTC().Add(-repository.lease)
	var tasks []models.Task
	findError := repository.database.WithContext(ctx).
		Where("scheduled_for <= ? AND retry_count < ? AND (state IN ? OR (state = ? AND last_attempted_at <= ?))", now.UTC(), maxRetries, []models.TaskState{models.TaskPending, models.TaskFailed}, models.TaskRunning, staleBefore).
		Order("scheduled_for ASC, created_at ASC, id ASC").
		Find(&tasks).Error
	if findError != nil {
		return nil, fmt.Errorf("read pending tasks: %w", findError)
	}
	jobs := make([]utilsscheduler.Job, 0, len(tasks))
	for taskIndex := range tasks {
		task := &tasks[taskIndex]
		scheduledFor := task.ScheduledFor.UTC()
		job := utilsscheduler.Job{
			ID:           task.ID,
			ScheduledFor: &scheduledFor,
			RetryCount:   task.RetryCount,
			Payload: taskPayload{
				OrganizerID:  task.OrganizerID,
				Kind:         task.Kind,
				ResourceType: task.ResourceType,
				ResourceID:   task.ResourceID,
			},
		}
		if task.LastAttemptedAt != nil {
			job.LastAttemptedAt = task.LastAttemptedAt.UTC()
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (repository *taskRepository) ClaimJobForAttempt(ctx context.Context, job utilsscheduler.Job, attemptedAt time.Time) (bool, error) {
	query := repository.database.WithContext(ctx).Table(config.TableTasks).
		Where("id = ? AND retry_count = ?", job.ID, job.RetryCount)
	if job.LastAttemptedAt.IsZero() {
		query = query.Where("state IN ? AND last_attempted_at IS NULL", []models.TaskState{models.TaskPending, models.TaskFailed})
	} else {
		query = query.Where("state IN ? AND last_attempted_at = ?", []models.TaskState{models.TaskPending, models.TaskRunning, models.TaskFailed}, job.LastAttemptedAt.UTC())
	}
	claim := query.Updates(map[string]any{
		"state":             models.TaskRunning,
		"last_attempted_at": attemptedAt.UTC(),
		"finished_at":       nil,
		"error_code":        nil,
		"updated_at":        attemptedAt.UTC(),
	})
	if claim.Error != nil {
		return false, fmt.Errorf("claim task %s: %w", job.ID, claim.Error)
	}
	return claim.RowsAffected == 1, nil
}

func (repository *taskRepository) ApplyAttemptResult(ctx context.Context, job utilsscheduler.Job, update utilsscheduler.AttemptUpdate) error {
	state := models.TaskState(update.Status)
	if state != models.TaskSucceeded && state != models.TaskFailed {
		return fmt.Errorf("task %s has invalid attempt state %q", job.ID, update.Status)
	}
	values := map[string]any{
		"state":             state,
		"retry_count":       update.RetryCount,
		"last_attempted_at": update.LastAttemptedAt.UTC(),
		"finished_at":       update.LastAttemptedAt.UTC(),
		"updated_at":        update.LastAttemptedAt.UTC(),
		"error_code":        nil,
	}
	if state == models.TaskFailed {
		values["error_code"] = calendarImportFailedCode
	}
	result := repository.database.WithContext(ctx).Table(config.TableTasks).
		Where("id = ? AND state = ? AND retry_count = ? AND last_attempted_at = ?", job.ID, models.TaskRunning, job.RetryCount, update.LastAttemptedAt.UTC()).
		Updates(values)
	if result.Error != nil {
		return fmt.Errorf("store task %s attempt: %w", job.ID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("store task %s attempt: task claim changed", job.ID)
	}
	return nil
}

type calendarConnectionTaskDispatcher struct {
	connectionService *CalendarConnectionService
	syncService       *CalendarSyncService
}

func (dispatcher *calendarConnectionTaskDispatcher) Attempt(ctx context.Context, job utilsscheduler.Job) (utilsscheduler.DispatchResult, error) {
	payload, valid := job.Payload.(taskPayload)
	if !valid || payload.Kind != models.TaskKindCalendarConnectionImport || payload.ResourceType != models.TaskResourceCalendarConnection {
		return utilsscheduler.DispatchResult{}, errors.New("calendar connection task payload is invalid")
	}
	syncStates, reconciliationError := dispatcher.connectionService.ReconcileSourceCalendars(ctx, payload.OrganizerID, payload.ResourceID)
	if reconciliationError != nil {
		return utilsscheduler.DispatchResult{}, reconciliationError
	}
	if synchronizationError := dispatcher.syncService.SynchronizeSyncStates(ctx, payload.OrganizerID, syncStates); synchronizationError != nil {
		return utilsscheduler.DispatchResult{}, synchronizationError
	}
	return utilsscheduler.DispatchResult{Status: string(models.TaskSucceeded)}, nil
}
