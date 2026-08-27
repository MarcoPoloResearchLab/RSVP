package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

type IngestionDraftStatus string
type IngestionDraftMode string

const (
	IngestionDraftIncomplete IngestionDraftStatus = "incomplete"
	IngestionDraftReady      IngestionDraftStatus = "ready"
	IngestionDraftConfirmed  IngestionDraftStatus = "confirmed"
	IngestionDraftCanceled   IngestionDraftStatus = "canceled"
	IngestionModeDatedEvent  IngestionDraftMode   = "dated_event"
	IngestionModeOpenLane    IngestionDraftMode   = "open_lane"
)

var (
	ErrIngestionDraftInvalid    = errors.New("ingestion draft is invalid")
	ErrDraftConfirmationInvalid = errors.New("draft confirmation is invalid")
)

// IngestionDraft stores proposed temporal data before explicit confirmation.
type IngestionDraft struct {
	BaseModel
	OrganizerID               string               `gorm:"type:varchar(8);not null;index"`
	Status                    IngestionDraftStatus `gorm:"type:text;not null;check:ingestion_draft_status,status IN ('incomplete','ready','confirmed','canceled')"`
	Mode                      IngestionDraftMode   `gorm:"type:text;not null;check:ingestion_draft_mode,mode IN ('dated_event','open_lane')"`
	CalendarID                string               `gorm:"type:varchar(8);not null;index"`
	Title                     string               `gorm:"not null"`
	AnchorEventID             *string              `gorm:"type:varchar(8);index"`
	StartsAt                  *time.Time
	EndsAt                    *time.Time
	ReviewIntervalSeconds     *int64
	NextProbeAt               *time.Time
	EscalationIntervalSeconds *int64
	ReferenceTime             time.Time          `gorm:"not null"`
	Timezone                  string             `gorm:"type:text;not null"`
	Organizer                 User               `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Calendar                  Calendar           `gorm:"foreignKey:CalendarID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Confirmation              *DraftConfirmation `gorm:"foreignKey:DraftID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type IngestionDraftProposal struct {
	Mode                      IngestionDraftMode
	CalendarID                string
	Title                     string
	AnchorEventID             *string
	StartsAt                  *time.Time
	EndsAt                    *time.Time
	ReviewIntervalSeconds     *int64
	NextProbeAt               *time.Time
	EscalationIntervalSeconds *int64
	ReferenceTime             time.Time
	Timezone                  string
}

func NewIngestionDraft(organizerID string, proposal IngestionDraftProposal) (*IngestionDraft, error) {
	draft := &IngestionDraft{OrganizerID: organizerID, Status: IngestionDraftReady, Mode: proposal.Mode, CalendarID: proposal.CalendarID, Title: proposal.Title, AnchorEventID: proposal.AnchorEventID, StartsAt: canonicalTime(proposal.StartsAt), EndsAt: canonicalTime(proposal.EndsAt), ReviewIntervalSeconds: proposal.ReviewIntervalSeconds, NextProbeAt: canonicalTime(proposal.NextProbeAt), EscalationIntervalSeconds: proposal.EscalationIntervalSeconds, ReferenceTime: proposal.ReferenceTime.UTC(), Timezone: proposal.Timezone}
	if validationError := draft.Validate(); validationError != nil {
		return nil, validationError
	}
	return draft, nil
}

func (draft *IngestionDraft) Validate() error {
	if draft.OrganizerID == "" || draft.CalendarID == "" || draft.Title == "" || draft.ReferenceTime.IsZero() {
		return ErrIngestionDraftInvalid
	}
	if _, timezoneError := NewTimezone(draft.Timezone); timezoneError != nil {
		return timezoneError
	}
	if draft.Status != IngestionDraftIncomplete && draft.Status != IngestionDraftReady && draft.Status != IngestionDraftConfirmed && draft.Status != IngestionDraftCanceled {
		return ErrIngestionDraftInvalid
	}
	if draft.Status == IngestionDraftIncomplete {
		return nil
	}
	switch draft.Mode {
	case IngestionModeDatedEvent:
		if draft.StartsAt == nil || draft.EndsAt == nil || !draft.EndsAt.After(*draft.StartsAt) || draft.ReviewIntervalSeconds != nil || draft.NextProbeAt != nil || draft.EscalationIntervalSeconds != nil {
			return ErrIngestionDraftInvalid
		}
	case IngestionModeOpenLane:
		if draft.AnchorEventID != nil || draft.StartsAt != nil || draft.EndsAt != nil {
			return ErrIngestionDraftInvalid
		}
		if (draft.ReviewIntervalSeconds == nil) != (draft.NextProbeAt == nil) {
			return ErrIngestionDraftInvalid
		}
		if draft.ReviewIntervalSeconds != nil && (*draft.ReviewIntervalSeconds <= 0 || draft.NextProbeAt.IsZero()) {
			return ErrIngestionDraftInvalid
		}
		if draft.EscalationIntervalSeconds != nil && *draft.EscalationIntervalSeconds <= 0 {
			return ErrIngestionDraftInvalid
		}
	default:
		return ErrIngestionDraftInvalid
	}
	return nil
}

func canonicalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	canonical := value.UTC()
	return &canonical
}
func (draft *IngestionDraft) BeforeCreate(database *gorm.DB) error {
	if err := draft.Validate(); err != nil {
		return err
	}
	return draft.BaseModel.GenerateID(database, draft)
}
func (draft *IngestionDraft) BeforeUpdate(*gorm.DB) error                   { return draft.Validate() }
func (draft *IngestionDraft) GetTableName() string                          { return config.TableIngestionDrafts }
func (draft *IngestionDraft) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }

// DraftConfirmation records the temporal resources created from one draft.
type DraftConfirmation struct {
	BaseModel
	DraftID           string           `gorm:"type:varchar(8);not null;uniqueIndex"`
	LaneID            string           `gorm:"type:varchar(8);not null"`
	EventID           *string          `gorm:"type:varchar(8)"`
	AttentionPolicyID *string          `gorm:"type:varchar(8)"`
	Draft             IngestionDraft   `gorm:"foreignKey:DraftID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lane              Lane             `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Event             *Event           `gorm:"foreignKey:EventID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	AttentionPolicy   *AttentionPolicy `gorm:"foreignKey:AttentionPolicyID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func NewDraftConfirmation(draftID string, laneID string, eventID *string, policyID *string) (*DraftConfirmation, error) {
	confirmation := &DraftConfirmation{DraftID: draftID, LaneID: laneID, EventID: eventID, AttentionPolicyID: policyID}
	if validationError := confirmation.Validate(); validationError != nil {
		return nil, validationError
	}
	return confirmation, nil
}
func (confirmation *DraftConfirmation) Validate() error {
	if confirmation.DraftID == "" || confirmation.LaneID == "" {
		return ErrDraftConfirmationInvalid
	}
	return nil
}
func (confirmation *DraftConfirmation) BeforeCreate(database *gorm.DB) error {
	if err := confirmation.Validate(); err != nil {
		return err
	}
	return confirmation.BaseModel.GenerateID(database, confirmation)
}
func (confirmation *DraftConfirmation) BeforeUpdate(*gorm.DB) error { return confirmation.Validate() }
func (confirmation *DraftConfirmation) GetTableName() string        { return config.TableDraftConfirmations }
func (confirmation *DraftConfirmation) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}
