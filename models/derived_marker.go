package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

type DerivedAnchorType string
type DerivedAnchorEdge string

const (
	DerivedAnchorEvent   DerivedAnchorType = "event"
	DerivedAnchorProbe   DerivedAnchorType = "probe"
	DerivedAnchorDerived DerivedAnchorType = "derived"
	DerivedAnchorStart   DerivedAnchorEdge = "start"
	DerivedAnchorEnd     DerivedAnchorEdge = "end"
)

var (
	ErrDerivedMarkerRuleInvalid = errors.New("derived marker rule is invalid")
	ErrDerivedMarkerInvalid     = errors.New("derived marker is invalid")
	ErrDerivedMarkerImmutable   = errors.New("derived marker time cannot change directly")
)

// DerivedMarkerRule stores one typed offset from an anchor marker.
type DerivedMarkerRule struct {
	BaseModel
	LaneID        string            `gorm:"type:varchar(8);not null;index"`
	AnchorType    DerivedAnchorType `gorm:"type:text;not null;check:derived_anchor_type,anchor_type IN ('event','probe','derived')"`
	AnchorID      string            `gorm:"type:varchar(8);not null;index"`
	AnchorEdge    DerivedAnchorEdge `gorm:"type:text;not null;check:derived_anchor_edge,anchor_edge IN ('start','end')"`
	OffsetSeconds int64             `gorm:"not null"`
	Lane          Lane              `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Marker        *DerivedMarker    `gorm:"foreignKey:RuleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewDerivedMarkerRule(laneID string, anchorType DerivedAnchorType, anchorID string, anchorEdge DerivedAnchorEdge, offsetSeconds int64) (*DerivedMarkerRule, error) {
	rule := &DerivedMarkerRule{LaneID: laneID, AnchorType: anchorType, AnchorID: anchorID, AnchorEdge: anchorEdge, OffsetSeconds: offsetSeconds}
	if validationError := rule.Validate(); validationError != nil {
		return nil, validationError
	}
	return rule, nil
}
func (rule *DerivedMarkerRule) Validate() error {
	if rule.LaneID == "" || rule.AnchorID == "" {
		return ErrDerivedMarkerRuleInvalid
	}
	if rule.AnchorType != DerivedAnchorEvent && rule.AnchorType != DerivedAnchorProbe && rule.AnchorType != DerivedAnchorDerived {
		return ErrDerivedMarkerRuleInvalid
	}
	if rule.AnchorEdge != DerivedAnchorStart && rule.AnchorEdge != DerivedAnchorEnd {
		return ErrDerivedMarkerRuleInvalid
	}
	return nil
}
func (rule *DerivedMarkerRule) BeforeCreate(database *gorm.DB) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	return rule.BaseModel.GenerateID(database, rule)
}
func (rule *DerivedMarkerRule) BeforeUpdate(*gorm.DB) error { return rule.Validate() }
func (rule *DerivedMarkerRule) GetTableName() string        { return config.TableDerivedMarkerRules }
func (rule *DerivedMarkerRule) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// DerivedMarker stores one calculated point marker.
type DerivedMarker struct {
	BaseModel
	RuleID   string            `gorm:"type:varchar(8);not null;uniqueIndex"`
	LaneID   string            `gorm:"type:varchar(8);not null;index"`
	At       time.Time         `gorm:"not null"`
	Timezone string            `gorm:"type:text;not null"`
	Rule     DerivedMarkerRule `gorm:"foreignKey:RuleID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lane     Lane              `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func NewDerivedMarker(ruleID string, laneID string, at time.Time, timezone Timezone) (*DerivedMarker, error) {
	marker := &DerivedMarker{RuleID: ruleID, LaneID: laneID, At: at.UTC(), Timezone: timezone.String()}
	if validationError := marker.Validate(); validationError != nil {
		return nil, validationError
	}
	return marker, nil
}
func (marker *DerivedMarker) Validate() error {
	if marker.RuleID == "" || marker.LaneID == "" || marker.At.IsZero() {
		return ErrDerivedMarkerInvalid
	}
	if _, timezoneError := NewTimezone(marker.Timezone); timezoneError != nil {
		return timezoneError
	}
	return nil
}
func (marker *DerivedMarker) BeforeCreate(database *gorm.DB) error {
	if err := marker.Validate(); err != nil {
		return err
	}
	return marker.BaseModel.GenerateID(database, marker)
}
func (*DerivedMarker) BeforeUpdate(*gorm.DB) error                          { return ErrDerivedMarkerImmutable }
func (marker *DerivedMarker) GetTableName() string                          { return config.TableDerivedMarkers }
func (marker *DerivedMarker) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }
