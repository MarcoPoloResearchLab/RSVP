package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// ProbeState is one closed review state.
type ProbeState string

const (
	ProbeStatePending   ProbeState = "pending"
	ProbeStateCompleted ProbeState = "completed"
	ProbeStateMissed    ProbeState = "missed"
	ProbeStateCanceled  ProbeState = "canceled"
)

var (
	ErrPolicyIDRequired  = errors.New("attention policy ID is required")
	ErrProbeDueRequired  = errors.New("probe due time is required")
	ErrProbeStateInvalid = errors.New("probe state is invalid")
	ErrProbeLaneMismatch = errors.New("probe lane must match its attention policy lane")
)

// Probe is one dated review action for a lane.
type Probe struct {
	BaseModel
	PolicyID    string    `gorm:"type:varchar(8);not null;index;uniqueIndex:probe_occurrence"`
	LaneID      string    `gorm:"type:varchar(8);not null;index"`
	DueAt       time.Time `gorm:"not null;uniqueIndex:probe_occurrence"`
	EscalatesAt *time.Time
	State       ProbeState `gorm:"type:text;not null;check:probe_state,((state = 'pending' AND completed_at IS NULL) OR (state = 'completed' AND completed_at IS NOT NULL) OR (state IN ('missed','canceled') AND completed_at IS NULL))"`
	CompletedAt *time.Time
	Policy      AttentionPolicy `gorm:"foreignKey:PolicyID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lane        Lane            `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewProbe constructs one valid pending probe.
func NewProbe(policyID string, laneID string, dueAt time.Time, escalatesAt *time.Time) (*Probe, error) {
	probe := &Probe{PolicyID: policyID, LaneID: laneID, DueAt: dueAt.UTC(), State: ProbeStatePending}
	if escalatesAt != nil {
		canonicalEscalation := escalatesAt.UTC()
		probe.EscalatesAt = &canonicalEscalation
	}
	if validationError := probe.Validate(); validationError != nil {
		return nil, validationError
	}
	return probe, nil
}

func (probe *Probe) Validate() error {
	if probe.PolicyID == "" {
		return ErrPolicyIDRequired
	}
	if probe.LaneID == "" {
		return ErrLaneIDRequired
	}
	if probe.DueAt.IsZero() {
		return ErrProbeDueRequired
	}
	if probe.EscalatesAt != nil && !probe.EscalatesAt.After(probe.DueAt) {
		return ErrProbeStateInvalid
	}
	switch probe.State {
	case ProbeStatePending:
		if probe.CompletedAt != nil {
			return ErrProbeStateInvalid
		}
	case ProbeStateCompleted:
		if probe.CompletedAt == nil {
			return ErrProbeStateInvalid
		}
	case ProbeStateMissed, ProbeStateCanceled:
		if probe.CompletedAt != nil {
			return ErrProbeStateInvalid
		}
	default:
		return ErrProbeStateInvalid
	}
	return nil
}

func (probe *Probe) validatePolicyLane(databaseConnection *gorm.DB) error {
	var policy AttentionPolicy
	if findError := databaseConnection.First(&policy, "id = ?", probe.PolicyID).Error; findError != nil {
		return findError
	}
	if policy.LaneID != probe.LaneID {
		return ErrProbeLaneMismatch
	}
	return nil
}

func (probe *Probe) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := probe.Validate(); validationError != nil {
		return validationError
	}
	if idError := probe.BaseModel.GenerateID(databaseConnection, probe); idError != nil {
		return idError
	}
	return probe.validatePolicyLane(databaseConnection)
}
func (probe *Probe) BeforeUpdate(databaseConnection *gorm.DB) error {
	if validationError := probe.Validate(); validationError != nil {
		return validationError
	}
	return probe.validatePolicyLane(databaseConnection)
}
func (probe *Probe) GetTableName() string                          { return config.TableProbes }
func (probe *Probe) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }
