package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// CalendarSyncState identifies one synchronization operation state.
type CalendarSyncState string

const (
	CalendarSyncPending   CalendarSyncState = "pending"
	CalendarSyncRunning   CalendarSyncState = "running"
	CalendarSyncSucceeded CalendarSyncState = "succeeded"
	CalendarSyncFailed    CalendarSyncState = "failed"
)

var (
	ErrExternalEventSeriesLinkInvalid = errors.New("external event series link is invalid")
	ErrExternalEventLinkInvalid       = errors.New("external event link is invalid")
	ErrCalendarSyncInvalid            = errors.New("calendar synchronization is invalid")
)

// ExternalEventSeriesLink connects one provider series to one RSVP event series.
type ExternalEventSeriesLink struct {
	BaseModel
	MappingID        string                `gorm:"type:varchar(8);not null;uniqueIndex:external_provider_series"`
	EventSeriesID    string                `gorm:"type:varchar(8);not null;uniqueIndex"`
	ProviderSeriesID string                `gorm:"not null;uniqueIndex:external_provider_series"`
	Mapping          SourceCalendarMapping `gorm:"foreignKey:MappingID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	EventSeries      EventSeries           `gorm:"foreignKey:EventSeriesID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewExternalEventSeriesLink(mappingID string, eventSeriesID string, providerSeriesID string) (*ExternalEventSeriesLink, error) {
	link := &ExternalEventSeriesLink{MappingID: mappingID, EventSeriesID: eventSeriesID, ProviderSeriesID: providerSeriesID}
	if validationError := link.Validate(); validationError != nil {
		return nil, validationError
	}
	return link, nil
}
func (link *ExternalEventSeriesLink) Validate() error {
	if link.MappingID == "" || link.EventSeriesID == "" || link.ProviderSeriesID == "" {
		return ErrExternalEventSeriesLinkInvalid
	}
	return nil
}
func (link *ExternalEventSeriesLink) BeforeCreate(database *gorm.DB) error {
	if err := link.Validate(); err != nil {
		return err
	}
	return link.BaseModel.GenerateID(database, link)
}
func (link *ExternalEventSeriesLink) BeforeUpdate(*gorm.DB) error { return link.Validate() }
func (link *ExternalEventSeriesLink) GetTableName() string {
	return config.TableExternalEventSeriesLinks
}
func (link *ExternalEventSeriesLink) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// ExternalEventLink connects one provider occurrence to one RSVP event.
type ExternalEventLink struct {
	BaseModel
	MappingID        string `gorm:"type:varchar(8);not null;uniqueIndex:external_provider_event"`
	EventID          string `gorm:"type:varchar(8);not null;uniqueIndex"`
	ProviderEventID  string `gorm:"not null;uniqueIndex:external_provider_event"`
	ProviderSeriesID *string
	Mapping          SourceCalendarMapping `gorm:"foreignKey:MappingID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Event            Event                 `gorm:"foreignKey:EventID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewExternalEventLink(mappingID string, eventID string, providerEventID string, providerSeriesID *string) (*ExternalEventLink, error) {
	link := &ExternalEventLink{MappingID: mappingID, EventID: eventID, ProviderEventID: providerEventID, ProviderSeriesID: providerSeriesID}
	if validationError := link.Validate(); validationError != nil {
		return nil, validationError
	}
	return link, nil
}
func (link *ExternalEventLink) Validate() error {
	if link.MappingID == "" || link.EventID == "" || link.ProviderEventID == "" {
		return ErrExternalEventLinkInvalid
	}
	return nil
}
func (link *ExternalEventLink) BeforeCreate(database *gorm.DB) error {
	if err := link.Validate(); err != nil {
		return err
	}
	return link.BaseModel.GenerateID(database, link)
}
func (link *ExternalEventLink) BeforeUpdate(*gorm.DB) error { return link.Validate() }
func (link *ExternalEventLink) GetTableName() string        { return config.TableExternalEventLinks }
func (link *ExternalEventLink) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// CalendarSync records one source calendar synchronization result.
type CalendarSync struct {
	BaseModel
	MappingID  string            `gorm:"type:varchar(8);not null;index"`
	State      CalendarSyncState `gorm:"type:text;not null;check:calendar_sync_state,state IN ('pending','running','succeeded','failed')"`
	StartedAt  time.Time         `gorm:"not null"`
	FinishedAt *time.Time
	ErrorCode  *string
	Mapping    SourceCalendarMapping `gorm:"foreignKey:MappingID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewCalendarSync(mappingID string, startedAt time.Time) (*CalendarSync, error) {
	sync := &CalendarSync{MappingID: mappingID, State: CalendarSyncPending, StartedAt: startedAt.UTC()}
	if validationError := sync.Validate(); validationError != nil {
		return nil, validationError
	}
	return sync, nil
}
func (sync *CalendarSync) Validate() error {
	if sync.MappingID == "" || sync.StartedAt.IsZero() {
		return ErrCalendarSyncInvalid
	}
	switch sync.State {
	case CalendarSyncPending, CalendarSyncRunning:
		if sync.FinishedAt != nil || sync.ErrorCode != nil {
			return ErrCalendarSyncInvalid
		}
	case CalendarSyncSucceeded:
		if sync.FinishedAt == nil || sync.ErrorCode != nil {
			return ErrCalendarSyncInvalid
		}
	case CalendarSyncFailed:
		if sync.FinishedAt == nil || sync.ErrorCode == nil || *sync.ErrorCode == "" {
			return ErrCalendarSyncInvalid
		}
	default:
		return ErrCalendarSyncInvalid
	}
	return nil
}
func (sync *CalendarSync) BeforeCreate(database *gorm.DB) error {
	if err := sync.Validate(); err != nil {
		return err
	}
	return sync.BaseModel.GenerateID(database, sync)
}
func (sync *CalendarSync) BeforeUpdate(*gorm.DB) error                   { return sync.Validate() }
func (sync *CalendarSync) GetTableName() string                          { return config.TableCalendarSyncs }
func (sync *CalendarSync) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }
