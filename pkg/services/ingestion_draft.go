package services

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const confirmIngestionDraftOperation = "confirm_ingestion_draft"

var ErrIngestionDraftNotReady = errors.New("ingestion draft is not ready for confirmation")

type IngestionDraftService struct {
	database *gorm.DB
	now      func() time.Time
}

func NewIngestionDraftService(database *gorm.DB, now func() time.Time) (*IngestionDraftService, error) {
	if database == nil || now == nil {
		return nil, errors.New("ingestion draft database and clock are required")
	}
	return &IngestionDraftService{database: database, now: now}, nil
}

func (service *IngestionDraftService) Create(ctx context.Context, organizerID string, proposal models.IngestionDraftProposal) (*models.IngestionDraft, error) {
	draft, draftError := models.NewIngestionDraft(organizerID, proposal)
	if draftError != nil {
		return nil, draftError
	}
	if relationshipError := validateDraftRelationships(service.database.WithContext(ctx), draft); relationshipError != nil {
		return nil, relationshipError
	}
	if createError := service.database.WithContext(ctx).Create(draft).Error; createError != nil {
		return nil, createError
	}
	return draft, nil
}

func (service *IngestionDraftService) Read(ctx context.Context, organizerID string, draftID string) (*models.IngestionDraft, error) {
	var draft models.IngestionDraft
	if findError := service.database.WithContext(ctx).Preload("Calendar").Preload("Confirmation").First(&draft, "id = ?", draftID).Error; findError != nil {
		return nil, findError
	}
	if draft.OrganizerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &draft, nil
}

func (service *IngestionDraftService) Update(ctx context.Context, organizerID string, draftID string, proposal models.IngestionDraftProposal) (*models.IngestionDraft, error) {
	var updated models.IngestionDraft
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updated, "id = ?", draftID).Error; findError != nil {
			return findError
		}
		if updated.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		if updated.Status != models.IngestionDraftReady && updated.Status != models.IngestionDraftIncomplete {
			return ErrIngestionDraftNotReady
		}
		candidate, candidateError := models.NewIngestionDraft(organizerID, proposal)
		if candidateError != nil {
			return candidateError
		}
		candidate.ID, candidate.CreatedAt = updated.ID, updated.CreatedAt
		if relationshipError := validateDraftRelationships(transaction, candidate); relationshipError != nil {
			return relationshipError
		}
		updated.Mode, updated.CalendarID, updated.Title, updated.AnchorEventID = candidate.Mode, candidate.CalendarID, candidate.Title, candidate.AnchorEventID
		updated.StartsAt, updated.EndsAt, updated.ReviewIntervalSeconds, updated.NextProbeAt = candidate.StartsAt, candidate.EndsAt, candidate.ReviewIntervalSeconds, candidate.NextProbeAt
		updated.EscalationIntervalSeconds, updated.ReferenceTime, updated.Timezone, updated.Status = candidate.EscalationIntervalSeconds, candidate.ReferenceTime, candidate.Timezone, models.IngestionDraftReady
		return transaction.Save(&updated).Error
	})
	return &updated, transactionError
}

func (service *IngestionDraftService) Cancel(ctx context.Context, organizerID string, draftID string) (*models.IngestionDraft, error) {
	var draft models.IngestionDraft
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&draft, "id = ?", draftID).Error; findError != nil {
			return findError
		}
		if draft.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		if draft.Status != models.IngestionDraftReady && draft.Status != models.IngestionDraftIncomplete {
			return ErrIngestionDraftNotReady
		}
		draft.Status = models.IngestionDraftCanceled
		return transaction.Save(&draft).Error
	})
	return &draft, transactionError
}

func (service *IngestionDraftService) Confirm(ctx context.Context, organizerID string, draftID string, idempotencyKey string) (*models.DraftConfirmation, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, ErrIdempotencyKeyRequired
	}
	payload, _ := json.Marshal(map[string]string{"draft_id": draftID})
	keyHash := sha256.Sum256([]byte(idempotencyKey))
	requestHash := sha256.Sum256(payload)
	if existing, found, lookupError := service.readIdempotentConfirmation(ctx, organizerID, keyHash[:], requestHash[:]); lookupError != nil || found {
		return existing, found, lookupError
	}
	var confirmation *models.DraftConfirmation
	reusedConfirmation := false
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if existing, found, lookupError := service.readIdempotentConfirmationWithDatabase(transaction, organizerID, keyHash[:], requestHash[:]); lookupError != nil || found {
			confirmation = existing
			reusedConfirmation = found
			return lookupError
		}
		var draft models.IngestionDraft
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&draft, "id = ?", draftID).Error; findError != nil {
			return findError
		}
		if draft.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		var existing models.DraftConfirmation
		if findError := transaction.First(&existing, "draft_id = ?", draft.ID).Error; findError == nil {
			confirmation = &existing
			reusedConfirmation = true
			return nil
		} else if !errors.Is(findError, gorm.ErrRecordNotFound) {
			return findError
		}
		if draft.Status != models.IngestionDraftReady {
			return ErrIngestionDraftNotReady
		}
		var organizer models.User
		if findError := transaction.First(&organizer, "id = ?", organizerID).Error; findError != nil {
			return findError
		}
		timezone, timezoneError := models.NewTimezone(draft.Timezone)
		if timezoneError != nil {
			return timezoneError
		}
		if confirmError := organizer.ConfirmTimezone(transaction, timezone); confirmError != nil {
			return confirmError
		}
		var laneID string
		var eventID *string
		var policyID *string
		switch draft.Mode {
		case models.IngestionModeOpenLane:
			order, orderError := models.NextLaneDisplayOrder(transaction, draft.CalendarID)
			if orderError != nil {
				return orderError
			}
			lane, laneError := models.NewOpenLane(draft.CalendarID, draft.Title, draft.ReferenceTime, order)
			if laneError != nil {
				return laneError
			}
			if createError := transaction.Create(lane).Error; createError != nil {
				return createError
			}
			laneID = lane.ID
			if draft.ReviewIntervalSeconds != nil {
				review := time.Duration(*draft.ReviewIntervalSeconds) * time.Second
				var escalation *time.Duration
				if draft.EscalationIntervalSeconds != nil {
					duration := time.Duration(*draft.EscalationIntervalSeconds) * time.Second
					escalation = &duration
				}
				policy, policyError := models.NewAttentionPolicy(lane.ID, review, *draft.NextProbeAt, escalation)
				if policyError != nil {
					return policyError
				}
				if createError := transaction.Create(policy).Error; createError != nil {
					return createError
				}
				if probeError := createPolicyProbe(transaction, policy); probeError != nil {
					return probeError
				}
				policyID = &policy.ID
			}
		case models.IngestionModeDatedEvent:
			event, eventError := models.CreateLocalIntervalEvent(transaction, &organizer, draft.CalendarID, draft.AnchorEventID, draft.Title, "", nil, *draft.StartsAt, *draft.EndsAt, draft.ReferenceTime, timezone)
			if eventError != nil {
				return eventError
			}
			laneID = event.LaneID
			eventID = &event.ID
		default:
			return models.ErrIngestionDraftInvalid
		}
		var confirmationError error
		confirmation, confirmationError = models.NewDraftConfirmation(draft.ID, laneID, eventID, policyID)
		if confirmationError != nil {
			return confirmationError
		}
		if createError := transaction.Create(confirmation).Error; createError != nil {
			return createError
		}
		draft.Status = models.IngestionDraftConfirmed
		if saveError := transaction.Save(&draft).Error; saveError != nil {
			return saveError
		}
		record, recordError := models.NewIdempotencyRecord(organizerID, confirmIngestionDraftOperation, keyHash[:], requestHash[:], http.StatusCreated, "draft_confirmation", confirmation.ID, service.now().UTC().Add(idempotencyLifetime))
		if recordError != nil {
			return recordError
		}
		return transaction.Create(record).Error
	})
	return confirmation, reusedConfirmation, transactionError
}

func validateDraftRelationships(database *gorm.DB, draft *models.IngestionDraft) error {
	var calendar models.Calendar
	if findError := database.First(&calendar, "id = ?", draft.CalendarID).Error; findError != nil {
		return findError
	}
	if calendar.OrganizerID != draft.OrganizerID {
		return ErrResourceForbidden
	}
	if draft.AnchorEventID != nil {
		var anchor models.Event
		if findError := anchor.FindByIDAndOwner(database, *draft.AnchorEventID, draft.OrganizerID); findError != nil {
			return findError
		}
		if anchor.Lane.CalendarID != draft.CalendarID || anchor.RelationType == models.EventRelationDependent {
			return models.ErrEventMembershipInvalid
		}
	}
	return nil
}

func (service *IngestionDraftService) readIdempotentConfirmation(ctx context.Context, organizerID string, keyHash []byte, requestHash []byte) (*models.DraftConfirmation, bool, error) {
	return service.readIdempotentConfirmationWithDatabase(service.database.WithContext(ctx), organizerID, keyHash, requestHash)
}

func (service *IngestionDraftService) readIdempotentConfirmationWithDatabase(database *gorm.DB, organizerID string, keyHash []byte, requestHash []byte) (*models.DraftConfirmation, bool, error) {
	var record models.IdempotencyRecord
	findError := database.First(&record, "organizer_id = ? AND operation = ? AND key_hash = ?", organizerID, confirmIngestionDraftOperation, keyHash).Error
	if errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if findError != nil {
		return nil, false, findError
	}
	if !record.ExpiresAt.After(service.now().UTC()) {
		if deleteError := database.Unscoped().Delete(&record).Error; deleteError != nil {
			return nil, false, deleteError
		}
		return nil, false, nil
	}
	if subtle.ConstantTimeCompare(record.RequestHash, requestHash) != 1 {
		return nil, false, ErrIdempotencyConflict
	}
	var confirmation models.DraftConfirmation
	if findError := database.First(&confirmation, "id = ?", record.ResourceID).Error; findError != nil {
		return nil, false, findError
	}
	return &confirmation, true, nil
}
