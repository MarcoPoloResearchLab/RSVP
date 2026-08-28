package models

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

type IngestionDraftStatus string
type IngestionDraftMode string
type IngestionDraftSource string

const (
	IngestionDraftIncomplete IngestionDraftStatus = "incomplete"
	IngestionDraftReady      IngestionDraftStatus = "ready"
	IngestionDraftConfirmed  IngestionDraftStatus = "confirmed"
	IngestionDraftCanceled   IngestionDraftStatus = "canceled"
	IngestionModeDatedEvent  IngestionDraftMode   = "dated_event"
	IngestionModeOpenLane    IngestionDraftMode   = "open_lane"
	IngestionSourceQuick     IngestionDraftSource = "quick"
	IngestionSourceNatural   IngestionDraftSource = "natural_language"
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
	Source                    IngestionDraftSource `gorm:"type:text;not null;check:ingestion_draft_source,source IN ('quick','natural_language')"`
	CalendarID                string               `gorm:"type:varchar(8);not null;index"`
	Title                     string               `gorm:"not null"`
	AnchorEventID             *string              `gorm:"type:varchar(8);index"`
	StartsAt                  *time.Time
	EndsAt                    *time.Time
	ReviewIntervalSeconds     *int64
	NextProbeAt               *time.Time
	EscalationIntervalSeconds *int64
	ReferenceTime             time.Time                `gorm:"not null"`
	Timezone                  string                   `gorm:"type:text;not null"`
	MissingFieldsJSON         string                   `gorm:"not null"`
	Organizer                 User                     `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Calendar                  Calendar                 `gorm:"foreignKey:CalendarID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Confirmation              *DraftConfirmation       `gorm:"foreignKey:DraftID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	DerivedMarkerRules        []DraftDerivedMarkerRule `gorm:"foreignKey:DraftID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
	draft := &IngestionDraft{OrganizerID: organizerID, Status: IngestionDraftReady, Mode: proposal.Mode, Source: IngestionSourceQuick, CalendarID: proposal.CalendarID, Title: proposal.Title, AnchorEventID: proposal.AnchorEventID, StartsAt: canonicalTime(proposal.StartsAt), EndsAt: canonicalTime(proposal.EndsAt), ReviewIntervalSeconds: proposal.ReviewIntervalSeconds, NextProbeAt: canonicalTime(proposal.NextProbeAt), EscalationIntervalSeconds: proposal.EscalationIntervalSeconds, ReferenceTime: proposal.ReferenceTime.UTC(), Timezone: proposal.Timezone, MissingFieldsJSON: "[]"}
	if validationError := draft.Validate(); validationError != nil {
		return nil, validationError
	}
	return draft, nil
}

func NewNaturalLanguageIngestionDraft(organizerID string, proposal IngestionDraftProposal, missingFields []string) (*IngestionDraft, error) {
	missingJSON, marshalError := json.Marshal(missingFields)
	if marshalError != nil {
		return nil, marshalError
	}
	status := IngestionDraftReady
	if len(missingFields) != 0 {
		status = IngestionDraftIncomplete
	}
	draft := &IngestionDraft{OrganizerID: organizerID, Status: status, Mode: proposal.Mode, Source: IngestionSourceNatural, CalendarID: proposal.CalendarID, Title: proposal.Title, AnchorEventID: proposal.AnchorEventID, StartsAt: canonicalTime(proposal.StartsAt), EndsAt: canonicalTime(proposal.EndsAt), ReviewIntervalSeconds: proposal.ReviewIntervalSeconds, NextProbeAt: canonicalTime(proposal.NextProbeAt), EscalationIntervalSeconds: proposal.EscalationIntervalSeconds, ReferenceTime: proposal.ReferenceTime.UTC(), Timezone: proposal.Timezone, MissingFieldsJSON: string(missingJSON)}
	if validationError := draft.Validate(); validationError != nil {
		return nil, validationError
	}
	return draft, nil
}

func (draft *IngestionDraft) MissingFields() ([]string, error) {
	var fields []string
	if unmarshalError := json.Unmarshal([]byte(draft.MissingFieldsJSON), &fields); unmarshalError != nil {
		return nil, ErrIngestionDraftInvalid
	}
	return fields, nil
}

func (draft *IngestionDraft) Validate() error {
	if draft.OrganizerID == "" || draft.CalendarID == "" || draft.ReferenceTime.IsZero() {
		return ErrIngestionDraftInvalid
	}
	if _, timezoneError := NewTimezone(draft.Timezone); timezoneError != nil {
		return timezoneError
	}
	if draft.Source != IngestionSourceQuick && draft.Source != IngestionSourceNatural {
		return ErrIngestionDraftInvalid
	}
	missingFields, missingError := draft.MissingFields()
	if missingError != nil {
		return missingError
	}
	allowedMissing := []string{"title", "starts_at", "ends_at", "next_probe_at"}
	for fieldIndex, field := range missingFields {
		if !slices.Contains(allowedMissing, field) || slices.Contains(missingFields[:fieldIndex], field) {
			return ErrIngestionDraftInvalid
		}
	}
	if draft.Status != IngestionDraftIncomplete && draft.Status != IngestionDraftReady && draft.Status != IngestionDraftConfirmed && draft.Status != IngestionDraftCanceled {
		return ErrIngestionDraftInvalid
	}
	if draft.Status == IngestionDraftIncomplete && len(missingFields) == 0 {
		return ErrIngestionDraftInvalid
	}
	if (draft.Status == IngestionDraftReady || draft.Status == IngestionDraftConfirmed) && len(missingFields) != 0 {
		return ErrIngestionDraftInvalid
	}
	if draft.Title == "" && !slices.Contains(missingFields, "title") {
		return ErrIngestionDraftInvalid
	}
	switch draft.Mode {
	case IngestionModeDatedEvent:
		if (draft.StartsAt == nil && !slices.Contains(missingFields, "starts_at")) || (draft.EndsAt == nil && !slices.Contains(missingFields, "ends_at")) || (draft.StartsAt != nil && draft.EndsAt != nil && !draft.EndsAt.After(*draft.StartsAt)) || draft.ReviewIntervalSeconds != nil || draft.NextProbeAt != nil || draft.EscalationIntervalSeconds != nil {
			return ErrIngestionDraftInvalid
		}
	case IngestionModeOpenLane:
		if draft.AnchorEventID != nil || draft.StartsAt != nil || draft.EndsAt != nil {
			return ErrIngestionDraftInvalid
		}
		if (draft.ReviewIntervalSeconds == nil) != (draft.NextProbeAt == nil) {
			if draft.ReviewIntervalSeconds == nil || draft.NextProbeAt != nil || !slices.Contains(missingFields, "next_probe_at") {
				return ErrIngestionDraftInvalid
			}
		}
		if draft.ReviewIntervalSeconds != nil && (*draft.ReviewIntervalSeconds <= 0 || (draft.NextProbeAt != nil && draft.NextProbeAt.IsZero())) {
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

// DraftDerivedMarkerRule stores one proposed rule until draft confirmation.
type DraftDerivedMarkerRule struct {
	BaseModel
	DraftID       string            `gorm:"type:varchar(8);not null;index;uniqueIndex:draft_rule_identity,priority:1"`
	AnchorEdge    DerivedAnchorEdge `gorm:"type:text;not null;check:draft_derived_anchor_edge,anchor_edge IN ('start','end');uniqueIndex:draft_rule_identity,priority:2"`
	OffsetSeconds int64             `gorm:"not null;uniqueIndex:draft_rule_identity,priority:3"`
	Draft         IngestionDraft    `gorm:"foreignKey:DraftID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewDraftDerivedMarkerRule(draftID string, anchorEdge DerivedAnchorEdge, offsetSeconds int64) (*DraftDerivedMarkerRule, error) {
	rule := &DraftDerivedMarkerRule{DraftID: draftID, AnchorEdge: anchorEdge, OffsetSeconds: offsetSeconds}
	if validationError := rule.Validate(); validationError != nil {
		return nil, validationError
	}
	return rule, nil
}
func (rule *DraftDerivedMarkerRule) Validate() error {
	if rule.DraftID == "" || (rule.AnchorEdge != DerivedAnchorStart && rule.AnchorEdge != DerivedAnchorEnd) {
		return ErrDerivedMarkerRuleInvalid
	}
	return nil
}
func (rule *DraftDerivedMarkerRule) BeforeCreate(database *gorm.DB) error {
	if validationError := rule.Validate(); validationError != nil {
		return validationError
	}
	return rule.BaseModel.GenerateID(database, rule)
}
func (rule *DraftDerivedMarkerRule) BeforeUpdate(*gorm.DB) error { return rule.Validate() }
func (rule *DraftDerivedMarkerRule) GetTableName() string        { return config.TableDraftDerivedMarkerRules }
func (rule *DraftDerivedMarkerRule) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

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
