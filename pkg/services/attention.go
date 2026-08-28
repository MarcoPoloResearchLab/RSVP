package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrAttentionLaneInvalid indicates that an attention policy does not address an active open lane.
	ErrAttentionLaneInvalid = errors.New("attention policy requires an active open lane")
	// ErrProbeTransitionInvalid indicates that a probe cannot use the requested transition.
	ErrProbeTransitionInvalid = errors.New("probe transition is invalid")
)

// AttentionPolicyPatch contains validated optional attention policy changes.
type AttentionPolicyPatch struct {
	ReviewInterval     *time.Duration
	NextProbeAt        *time.Time
	EscalationInterval **time.Duration
}

// AttentionService changes organizer-owned attention policies and probes.
type AttentionService struct {
	database *gorm.DB
	now      func() time.Time
}

// NewAttentionService constructs one attention service.
func NewAttentionService(database *gorm.DB, now func() time.Time) (*AttentionService, error) {
	if database == nil || now == nil {
		return nil, errors.New("attention database and clock are required")
	}
	return &AttentionService{database: database, now: now}, nil
}

// CreatePolicy creates one attention policy and its first pending probe.
func (service *AttentionService) CreatePolicy(ctx context.Context, organizerID string, laneID string, reviewInterval time.Duration, nextProbeAt time.Time, escalationInterval *time.Duration) (*models.AttentionPolicy, error) {
	var createdPolicy *models.AttentionPolicy
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		lane, laneError := attentionLane(transaction, organizerID, laneID)
		if laneError != nil {
			return laneError
		}
		if lane.Status != models.LaneStatusActive || lane.EndsAt != nil {
			return ErrAttentionLaneInvalid
		}
		policy, policyError := models.NewAttentionPolicy(laneID, reviewInterval, nextProbeAt, escalationInterval)
		if policyError != nil {
			return policyError
		}
		if createError := transaction.Create(policy).Error; createError != nil {
			return fmt.Errorf("create attention policy for lane %s: %w", laneID, createError)
		}
		if probeError := createPolicyProbe(transaction, policy); probeError != nil {
			return probeError
		}
		createdPolicy = policy
		return nil
	})
	return createdPolicy, transactionError
}

// ReadPolicy returns one organizer-owned attention policy and its probe history.
func (service *AttentionService) ReadPolicy(ctx context.Context, organizerID string, policyID string) (*models.AttentionPolicy, error) {
	var policy models.AttentionPolicy
	if findError := service.database.WithContext(ctx).Preload("Probes", func(database *gorm.DB) *gorm.DB {
		return database.Order("due_at ASC").Order("id ASC")
	}).First(&policy, "id = ?", policyID).Error; findError != nil {
		return nil, findError
	}
	if _, laneError := attentionLane(service.database.WithContext(ctx), organizerID, policy.LaneID); laneError != nil {
		return nil, laneError
	}
	return &policy, nil
}

// UpdatePolicy changes one policy and replaces its pending occurrence.
func (service *AttentionService) UpdatePolicy(ctx context.Context, organizerID string, policyID string, patch AttentionPolicyPatch) (*models.AttentionPolicy, error) {
	var updatedPolicy models.AttentionPolicy
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updatedPolicy, "id = ?", policyID).Error; findError != nil {
			return findError
		}
		lane, laneError := attentionLane(transaction, organizerID, updatedPolicy.LaneID)
		if laneError != nil {
			return laneError
		}
		if lane.Status != models.LaneStatusActive || lane.EndsAt != nil {
			return ErrAttentionLaneInvalid
		}
		if patch.ReviewInterval != nil {
			updatedPolicy.ReviewIntervalSeconds = int64(patch.ReviewInterval.Seconds())
		}
		if patch.NextProbeAt != nil {
			updatedPolicy.NextProbeAt = patch.NextProbeAt.UTC()
		}
		if patch.EscalationInterval != nil {
			updatedPolicy.EscalationIntervalSeconds = nil
			if *patch.EscalationInterval != nil {
				seconds := int64((**patch.EscalationInterval).Seconds())
				updatedPolicy.EscalationIntervalSeconds = &seconds
			}
		}
		if validationError := updatedPolicy.Validate(); validationError != nil {
			return validationError
		}
		if updateError := transaction.Model(&updatedPolicy).Updates(map[string]any{
			"review_interval_seconds":     updatedPolicy.ReviewIntervalSeconds,
			"next_probe_at":               updatedPolicy.NextProbeAt,
			"escalation_interval_seconds": updatedPolicy.EscalationIntervalSeconds,
		}).Error; updateError != nil {
			return fmt.Errorf("update attention policy %s: %w", policyID, updateError)
		}
		var pendingProbe models.Probe
		if findError := transaction.First(&pendingProbe, "policy_id = ? AND state = ?", policyID, models.ProbeStatePending).Error; findError != nil {
			return findError
		}
		pendingProbe.DueAt = updatedPolicy.NextProbeAt
		pendingProbe.EscalatesAt = policyEscalationTime(&updatedPolicy)
		if validationError := pendingProbe.Validate(); validationError != nil {
			return validationError
		}
		if updateError := transaction.Model(&pendingProbe).Updates(map[string]any{
			"due_at": pendingProbe.DueAt, "escalates_at": pendingProbe.EscalatesAt,
		}).Error; updateError != nil {
			return fmt.Errorf("update pending probe for policy %s: %w", policyID, updateError)
		}
		return nil
	})
	return &updatedPolicy, transactionError
}

// DeletePolicy deletes one organizer-owned attention policy and its probes.
func (service *AttentionService) DeletePolicy(ctx context.Context, organizerID string, policyID string) error {
	return service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var policy models.AttentionPolicy
		if findError := transaction.First(&policy, "id = ?", policyID).Error; findError != nil {
			return findError
		}
		if _, laneError := attentionLane(transaction, organizerID, policy.LaneID); laneError != nil {
			return laneError
		}
		if deleteError := transaction.Unscoped().Where("policy_id = ?", policyID).Delete(&models.Probe{}).Error; deleteError != nil {
			return fmt.Errorf("delete probes for policy %s: %w", policyID, deleteError)
		}
		if deleteError := transaction.Unscoped().Delete(&policy).Error; deleteError != nil {
			return fmt.Errorf("delete attention policy %s: %w", policyID, deleteError)
		}
		return nil
	})
}

// CompleteProbe records one completion and creates the next policy occurrence.
func (service *AttentionService) CompleteProbe(ctx context.Context, organizerID string, probeID string) (*models.Probe, error) {
	var completedProbe models.Probe
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&completedProbe, "id = ?", probeID).Error; findError != nil {
			return findError
		}
		if _, laneError := attentionLane(transaction, organizerID, completedProbe.LaneID); laneError != nil {
			return laneError
		}
		if completedProbe.State != models.ProbeStatePending {
			return ErrProbeTransitionInvalid
		}
		completionTime := service.now().UTC()
		completedProbe.State = models.ProbeStateCompleted
		completedProbe.CompletedAt = &completionTime
		if updateError := transaction.Model(&completedProbe).Updates(map[string]any{"state": completedProbe.State, "completed_at": completionTime}).Error; updateError != nil {
			return fmt.Errorf("complete probe %s: %w", probeID, updateError)
		}
		return service.scheduleNextProbe(transaction, &completedProbe, completionTime)
	})
	return &completedProbe, transactionError
}

// MarkMissedProbes records each organizer probe whose escalation time has passed.
func (service *AttentionService) MarkMissedProbes(ctx context.Context, organizerID string) (int, error) {
	missedCount := 0
	currentTime := service.now().UTC()
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var probes []models.Probe
		if findError := transaction.Model(&models.Probe{}).
			Joins("JOIN "+config.TableAttentionPolicies+" ON "+config.TableAttentionPolicies+".id = "+config.TableProbes+".policy_id AND "+config.TableAttentionPolicies+".deleted_at IS NULL").
			Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableProbes+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
			Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
			Where(config.TableCalendars+".organizer_id = ? AND "+config.TableProbes+".state = ? AND "+config.TableProbes+".escalates_at IS NOT NULL AND "+config.TableProbes+".escalates_at <= ?", organizerID, models.ProbeStatePending, currentTime).
			Order(config.TableProbes + ".escalates_at ASC").Find(&probes).Error; findError != nil {
			return fmt.Errorf("read overdue probes for organizer %s: %w", organizerID, findError)
		}
		for probeIndex := range probes {
			probe := &probes[probeIndex]
			if updateError := transaction.Model(probe).Updates(map[string]any{"state": models.ProbeStateMissed, "completed_at": nil}).Error; updateError != nil {
				return fmt.Errorf("mark probe %s missed: %w", probe.ID, updateError)
			}
			probe.State = models.ProbeStateMissed
			if scheduleError := service.scheduleNextProbe(transaction, probe, currentTime); scheduleError != nil {
				return scheduleError
			}
			missedCount++
		}
		return nil
	})
	return missedCount, transactionError
}

// MarkAllMissedProbes records each probe that the application clock identifies as missed.
func (service *AttentionService) MarkAllMissedProbes(ctx context.Context) (int, error) {
	currentTime := service.now().UTC()
	var organizerIDs []string
	queryError := service.database.WithContext(ctx).Model(&models.Probe{}).
		Distinct(config.TableCalendars+".organizer_id").
		Joins("JOIN "+config.TableLanes+" ON "+config.TableLanes+".id = "+config.TableProbes+".lane_id AND "+config.TableLanes+".deleted_at IS NULL").
		Joins("JOIN "+config.TableCalendars+" ON "+config.TableCalendars+".id = "+config.TableLanes+".calendar_id AND "+config.TableCalendars+".deleted_at IS NULL").
		Where(config.TableProbes+".state = ? AND "+config.TableProbes+".escalates_at IS NOT NULL AND "+config.TableProbes+".escalates_at <= ?", models.ProbeStatePending, currentTime).
		Pluck(config.TableCalendars+".organizer_id", &organizerIDs).Error
	if queryError != nil {
		return 0, fmt.Errorf("read organizers with overdue probes: %w", queryError)
	}
	total := 0
	for _, organizerID := range organizerIDs {
		count, missedError := service.MarkMissedProbes(ctx, organizerID)
		if missedError != nil {
			return total, missedError
		}
		total += count
	}
	return total, nil
}

func (service *AttentionService) scheduleNextProbe(transaction *gorm.DB, priorProbe *models.Probe, referenceTime time.Time) error {
	var policy models.AttentionPolicy
	if findError := transaction.First(&policy, "id = ?", priorProbe.PolicyID).Error; findError != nil {
		return findError
	}
	var lane models.Lane
	if findError := transaction.First(&lane, "id = ?", policy.LaneID).Error; findError != nil {
		return findError
	}
	if lane.Status != models.LaneStatusActive || lane.EndsAt != nil {
		return nil
	}
	policy.NextProbeAt = referenceTime.UTC().Add(time.Duration(policy.ReviewIntervalSeconds) * time.Second)
	if updateError := transaction.Model(&policy).Update("next_probe_at", policy.NextProbeAt).Error; updateError != nil {
		return fmt.Errorf("set next probe time for policy %s: %w", policy.ID, updateError)
	}
	return createPolicyProbe(transaction, &policy)
}

func createPolicyProbe(database *gorm.DB, policy *models.AttentionPolicy) error {
	escalatesAt := policyEscalationTime(policy)
	probe, probeError := models.NewProbe(policy.ID, policy.LaneID, policy.NextProbeAt, escalatesAt)
	if probeError != nil {
		return probeError
	}
	if createError := database.Create(probe).Error; createError != nil {
		return fmt.Errorf("create pending probe for policy %s: %w", policy.ID, createError)
	}
	return nil
}

func policyEscalationTime(policy *models.AttentionPolicy) *time.Time {
	if policy.EscalationIntervalSeconds == nil {
		return nil
	}
	value := policy.NextProbeAt.Add(time.Duration(*policy.EscalationIntervalSeconds) * time.Second)
	return &value
}

func attentionLane(database *gorm.DB, organizerID string, laneID string) (*models.Lane, error) {
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
		return nil, findError
	}
	if ownerError := requireCalendarOwner(database, lane.CalendarID, organizerID); ownerError != nil {
		return nil, ownerError
	}
	return &lane, nil
}
