package models

import (
	"errors"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// EventSourceKind identifies the owner of event series data.
type EventSourceKind string

const (
	EventSourceLocal  EventSourceKind = "local"
	EventSourceGoogle EventSourceKind = "google"
)

var (
	ErrEventSeriesIDRequired  = errors.New("event series ID is required")
	ErrEventSourceKindInvalid = errors.New("event source kind is invalid")
)

// EventSeries groups event occurrences on one lane.
type EventSeries struct {
	BaseModel
	LaneID         string          `gorm:"type:varchar(8);not null;uniqueIndex"`
	Timezone       string          `gorm:"type:text;not null"`
	SourceKind     EventSourceKind `gorm:"type:text;not null;check:event_series_source_kind,source_kind IN ('local','google')"`
	RecurrenceRule *string         `gorm:"type:text"`
	Lane           Lane            `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Events         []Event         `gorm:"foreignKey:EventSeriesID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewEventSeries constructs one valid event series.
func NewEventSeries(laneID string, timezone Timezone, sourceKind EventSourceKind, recurrenceRule *string) (*EventSeries, error) {
	series := &EventSeries{LaneID: laneID, Timezone: timezone.String(), SourceKind: sourceKind, RecurrenceRule: recurrenceRule}
	if validationError := series.Validate(); validationError != nil {
		return nil, validationError
	}
	return series, nil
}

// Validate checks the event series invariants.
func (series *EventSeries) Validate() error {
	if series.LaneID == "" {
		return ErrLaneIDRequired
	}
	if _, timezoneError := NewTimezone(series.Timezone); timezoneError != nil {
		return timezoneError
	}
	if series.SourceKind != EventSourceLocal && series.SourceKind != EventSourceGoogle {
		return ErrEventSourceKindInvalid
	}
	return nil
}

func (series *EventSeries) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := series.Validate(); validationError != nil {
		return validationError
	}
	return series.BaseModel.GenerateID(databaseConnection, series)
}

func (series *EventSeries) BeforeUpdate(*gorm.DB) error                   { return series.Validate() }
func (series *EventSeries) GetTableName() string                          { return config.TableEventSeries }
func (series *EventSeries) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }
