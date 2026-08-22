package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

var (
	ErrReviewIntervalInvalid     = errors.New("review interval must be positive")
	ErrNextProbeRequired         = errors.New("next probe time is required")
	ErrEscalationIntervalInvalid = errors.New("escalation interval must be positive")
)

// AttentionPolicy specifies the next probe and optional escalation interval for a lane.
type AttentionPolicy struct {
	BaseModel
	LaneID                    string    `gorm:"type:varchar(8);not null;uniqueIndex"`
	ReviewIntervalSeconds     int64     `gorm:"not null;check:attention_review_interval,review_interval_seconds > 0"`
	NextProbeAt               time.Time `gorm:"not null"`
	EscalationIntervalSeconds *int64    `gorm:"check:attention_escalation_interval,escalation_interval_seconds IS NULL OR escalation_interval_seconds > 0"`
	Lane                      Lane      `gorm:"foreignKey:LaneID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Probes                    []Probe   `gorm:"foreignKey:PolicyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewAttentionPolicy constructs one valid attention policy.
func NewAttentionPolicy(laneID string, reviewInterval time.Duration, nextProbeAt time.Time, escalationInterval *time.Duration) (*AttentionPolicy, error) {
	policy := &AttentionPolicy{LaneID: laneID, ReviewIntervalSeconds: int64(reviewInterval.Seconds()), NextProbeAt: nextProbeAt.UTC()}
	if escalationInterval != nil {
		seconds := int64(escalationInterval.Seconds())
		policy.EscalationIntervalSeconds = &seconds
	}
	if validationError := policy.Validate(); validationError != nil {
		return nil, validationError
	}
	return policy, nil
}

func (policy *AttentionPolicy) Validate() error {
	if policy.LaneID == "" {
		return ErrLaneIDRequired
	}
	if policy.ReviewIntervalSeconds <= 0 {
		return ErrReviewIntervalInvalid
	}
	if policy.NextProbeAt.IsZero() {
		return ErrNextProbeRequired
	}
	if policy.EscalationIntervalSeconds != nil && *policy.EscalationIntervalSeconds <= 0 {
		return ErrEscalationIntervalInvalid
	}
	return nil
}

func (policy *AttentionPolicy) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := policy.Validate(); validationError != nil {
		return validationError
	}
	return policy.BaseModel.GenerateID(databaseConnection, policy)
}
func (policy *AttentionPolicy) BeforeUpdate(*gorm.DB) error { return policy.Validate() }
func (policy *AttentionPolicy) GetTableName() string        { return config.TableAttentionPolicies }
func (policy *AttentionPolicy) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}
