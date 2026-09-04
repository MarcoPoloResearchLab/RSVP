package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// TaskKind identifies one closed background operation.
type TaskKind string

// TaskResourceType identifies the resource that owns a task.
type TaskResourceType string

// TaskState identifies one closed task state.
type TaskState string

const (
	// TaskKindCalendarConnectionImport imports calendars and events for one connection.
	TaskKindCalendarConnectionImport TaskKind = "calendar_connection_import"
	// TaskResourceCalendarConnection identifies a calendar connection task resource.
	TaskResourceCalendarConnection TaskResourceType = "calendar_connection"
	// TaskPending identifies a task that is ready for its first attempt.
	TaskPending TaskState = "pending"
	// TaskRunning identifies a claimed task attempt.
	TaskRunning TaskState = "running"
	// TaskSucceeded identifies a completed task.
	TaskSucceeded TaskState = "succeeded"
	// TaskFailed identifies a failed task that can have another attempt.
	TaskFailed TaskState = "failed"
)

var ErrTaskInvalid = errors.New("task is invalid")

// Task stores one durable background operation without private provider data.
type Task struct {
	BaseModel
	OrganizerID     string           `gorm:"type:varchar(8);not null;index"`
	Kind            TaskKind         `gorm:"type:text;not null;uniqueIndex:task_resource,priority:1;check:task_kind,kind = 'calendar_connection_import'"`
	ResourceType    TaskResourceType `gorm:"type:text;not null;uniqueIndex:task_resource,priority:2;check:task_resource_type,resource_type = 'calendar_connection'"`
	ResourceID      string           `gorm:"type:varchar(8);not null;uniqueIndex:task_resource,priority:3"`
	State           TaskState        `gorm:"type:text;not null;check:task_state,state IN ('pending','running','succeeded','failed')"`
	ScheduledFor    time.Time        `gorm:"not null"`
	RetryCount      int              `gorm:"not null;check:task_retry_count,retry_count >= 0"`
	LastAttemptedAt *time.Time
	FinishedAt      *time.Time
	ErrorCode       *string
	Organizer       User `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewCalendarConnectionImportTask constructs one ready calendar import task.
func NewCalendarConnectionImportTask(organizerID string, connectionID string, scheduledFor time.Time) (*Task, error) {
	task := &Task{
		OrganizerID:  organizerID,
		Kind:         TaskKindCalendarConnectionImport,
		ResourceType: TaskResourceCalendarConnection,
		ResourceID:   connectionID,
		State:        TaskPending,
		ScheduledFor: scheduledFor.UTC(),
	}
	if validationError := task.Validate(); validationError != nil {
		return nil, validationError
	}
	return task, nil
}

// Validate confirms the closed task contract.
func (task *Task) Validate() error {
	if task.OrganizerID == "" || task.Kind != TaskKindCalendarConnectionImport || task.ResourceType != TaskResourceCalendarConnection || task.ResourceID == "" || task.ScheduledFor.IsZero() || task.RetryCount < 0 {
		return ErrTaskInvalid
	}
	if task.State != TaskPending && task.State != TaskRunning && task.State != TaskSucceeded && task.State != TaskFailed {
		return ErrTaskInvalid
	}
	switch task.State {
	case TaskPending:
		if task.RetryCount != 0 || task.LastAttemptedAt != nil || task.FinishedAt != nil || task.ErrorCode != nil {
			return ErrTaskInvalid
		}
	case TaskRunning:
		if task.LastAttemptedAt == nil || task.FinishedAt != nil || task.ErrorCode != nil {
			return ErrTaskInvalid
		}
	case TaskSucceeded:
		if task.RetryCount == 0 || task.LastAttemptedAt == nil || task.FinishedAt == nil || task.ErrorCode != nil {
			return ErrTaskInvalid
		}
	case TaskFailed:
		if task.RetryCount == 0 || task.LastAttemptedAt == nil || task.FinishedAt == nil || task.ErrorCode == nil {
			return ErrTaskInvalid
		}
	}
	return nil
}

func (task *Task) BeforeCreate(database *gorm.DB) error {
	if validationError := task.Validate(); validationError != nil {
		return validationError
	}
	return task.BaseModel.GenerateID(database, task)
}

func (task *Task) GetTableName() string { return config.TableTasks }
func (task *Task) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}
