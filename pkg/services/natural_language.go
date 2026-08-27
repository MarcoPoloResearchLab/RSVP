package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
)

var (
	ErrNaturalLanguageInputRequired           = errors.New("natural-language input is required")
	ErrNaturalLanguageUnavailable             = errors.New("natural-language parsing is unavailable")
	ErrNaturalLanguageProviderFailed          = errors.New("natural-language provider failed")
	ErrNaturalLanguageProviderResponseInvalid = errors.New("natural-language provider response is invalid")
)

type NaturalLanguageParseRequest struct {
	InputText     string    `json:"input_text"`
	ReferenceTime time.Time `json:"reference_time"`
	Timezone      string    `json:"timezone"`
}

type NaturalLanguageRelativeRule struct {
	AnchorEdge    models.DerivedAnchorEdge `json:"anchor_edge"`
	OffsetSeconds int64                    `json:"offset_seconds"`
}

type NaturalLanguageParseResponse struct {
	Mode                      models.IngestionDraftMode     `json:"mode"`
	Title                     string                        `json:"title"`
	AnchorEventID             *string                       `json:"anchor_event_id"`
	StartsAt                  *string                       `json:"starts_at"`
	EndsAt                    *string                       `json:"ends_at"`
	ReviewIntervalSeconds     *int64                        `json:"review_interval_seconds"`
	NextProbeAt               *string                       `json:"next_probe_at"`
	EscalationIntervalSeconds *int64                        `json:"escalation_interval_seconds"`
	RelativeRules             []NaturalLanguageRelativeRule `json:"relative_rules"`
}

type NaturalLanguageParser interface {
	Parse(context.Context, NaturalLanguageParseRequest) (NaturalLanguageParseResponse, error)
}

type NaturalLanguageService struct {
	database *gorm.DB
	parser   NaturalLanguageParser
}

func NewNaturalLanguageService(database *gorm.DB, parser NaturalLanguageParser) (*NaturalLanguageService, error) {
	if database == nil || parser == nil {
		return nil, errors.New("natural-language database and parser are required")
	}
	return &NaturalLanguageService{database: database, parser: parser}, nil
}

func (service *NaturalLanguageService) Parse(ctx context.Context, organizerID string, inputText string, calendarID string, referenceTime time.Time, timezone models.Timezone) (*models.IngestionDraft, error) {
	if strings.TrimSpace(inputText) == "" {
		return nil, ErrNaturalLanguageInputRequired
	}
	providerResponse, parserError := service.parser.Parse(ctx, NaturalLanguageParseRequest{InputText: inputText, ReferenceTime: referenceTime.UTC(), Timezone: timezone.String()})
	if parserError != nil {
		if errors.Is(parserError, ErrNaturalLanguageProviderResponseInvalid) {
			return nil, ErrNaturalLanguageProviderResponseInvalid
		}
		return nil, ErrNaturalLanguageProviderFailed
	}
	proposal, missingFields, validationError := validateNaturalLanguageResponse(providerResponse, calendarID, referenceTime, timezone)
	if validationError != nil {
		return nil, validationError
	}
	var draft *models.IngestionDraft
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var draftError error
		draft, draftError = models.NewNaturalLanguageIngestionDraft(organizerID, proposal, missingFields)
		if draftError != nil {
			return fmt.Errorf("%w: %v", ErrNaturalLanguageProviderResponseInvalid, draftError)
		}
		if relationshipError := validateDraftRelationships(transaction, draft); relationshipError != nil {
			return relationshipError
		}
		if createError := transaction.Create(draft).Error; createError != nil {
			return createError
		}
		for ruleIndex := range providerResponse.RelativeRules {
			providerRule := providerResponse.RelativeRules[ruleIndex]
			rule, ruleError := models.NewDraftDerivedMarkerRule(draft.ID, providerRule.AnchorEdge, providerRule.OffsetSeconds)
			if ruleError != nil {
				return fmt.Errorf("%w: relative rule", ErrNaturalLanguageProviderResponseInvalid)
			}
			if createError := transaction.Create(rule).Error; createError != nil {
				return createError
			}
			draft.DerivedMarkerRules = append(draft.DerivedMarkerRules, *rule)
		}
		return nil
	})
	return draft, transactionError
}

func validateNaturalLanguageResponse(response NaturalLanguageParseResponse, calendarID string, referenceTime time.Time, timezone models.Timezone) (models.IngestionDraftProposal, []string, error) {
	if calendarID == "" || referenceTime.IsZero() || (response.Mode != models.IngestionModeDatedEvent && response.Mode != models.IngestionModeOpenLane) {
		return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
	}
	startsAt, startsError := parseNaturalLanguageTime(response.StartsAt, timezone)
	if startsError != nil {
		return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
	}
	endsAt, endsError := parseNaturalLanguageTime(response.EndsAt, timezone)
	if endsError != nil {
		return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
	}
	nextProbeAt, probeError := parseNaturalLanguageTime(response.NextProbeAt, timezone)
	if probeError != nil {
		return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
	}
	missingFields := make([]string, 0, 3)
	if strings.TrimSpace(response.Title) == "" {
		missingFields = append(missingFields, "title")
	}
	if response.Mode == models.IngestionModeDatedEvent {
		if startsAt == nil {
			missingFields = append(missingFields, "starts_at")
		}
		if endsAt == nil {
			missingFields = append(missingFields, "ends_at")
		}
		if response.ReviewIntervalSeconds != nil || nextProbeAt != nil || response.EscalationIntervalSeconds != nil {
			return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
		}
	} else {
		if startsAt != nil || endsAt != nil || response.AnchorEventID != nil || len(response.RelativeRules) != 0 {
			return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
		}
		if response.ReviewIntervalSeconds != nil && nextProbeAt == nil {
			missingFields = append(missingFields, "next_probe_at")
		}
		if response.ReviewIntervalSeconds == nil && nextProbeAt != nil {
			return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
		}
	}
	seenRules := make([]NaturalLanguageRelativeRule, 0, len(response.RelativeRules))
	for _, rule := range response.RelativeRules {
		if (rule.AnchorEdge != models.DerivedAnchorStart && rule.AnchorEdge != models.DerivedAnchorEnd) || slices.Contains(seenRules, rule) {
			return models.IngestionDraftProposal{}, nil, ErrNaturalLanguageProviderResponseInvalid
		}
		seenRules = append(seenRules, rule)
	}
	proposal := models.IngestionDraftProposal{Mode: response.Mode, CalendarID: calendarID, Title: strings.TrimSpace(response.Title), AnchorEventID: response.AnchorEventID, StartsAt: startsAt, EndsAt: endsAt, ReviewIntervalSeconds: response.ReviewIntervalSeconds, NextProbeAt: nextProbeAt, EscalationIntervalSeconds: response.EscalationIntervalSeconds, ReferenceTime: referenceTime.UTC(), Timezone: timezone.String()}
	return proposal, missingFields, nil
}

func parseNaturalLanguageTime(value *string, timezone models.Timezone) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if instant, parseError := time.Parse(time.RFC3339Nano, *value); parseError == nil {
		canonical := instant.UTC()
		return &canonical, nil
	}
	instant, parseError := models.ParseLocalTime(*value, "2006-01-02T15:04", timezone)
	if parseError != nil {
		return nil, parseError
	}
	return &instant, nil
}
