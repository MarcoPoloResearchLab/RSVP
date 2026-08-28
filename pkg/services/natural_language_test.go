package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
)

type deterministicNaturalLanguageParser struct {
	responses map[string]services.NaturalLanguageParseResponse
	requests  []services.NaturalLanguageParseRequest
}

func (parser *deterministicNaturalLanguageParser) Parse(_ context.Context, request services.NaturalLanguageParseRequest) (services.NaturalLanguageParseResponse, error) {
	parser.requests = append(parser.requests, request)
	response, found := parser.responses[request.InputText]
	if !found {
		return services.NaturalLanguageParseResponse{}, errors.New("deterministic parser fixture absent")
	}
	return response, nil
}

func TestNaturalLanguageWaitingDraftCreatesAttentionOnlyAfterConfirmation(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, anchor)
	referenceTime := testsupport.FixedStartTime()
	nextProbe := referenceTime.Add(7 * 24 * time.Hour).Format(time.RFC3339Nano)
	reviewSeconds := int64(604800)
	parser := &deterministicNaturalLanguageParser{responses: map[string]services.NaturalLanguageParseResponse{
		"unresolved appeal": {Mode: models.IngestionModeOpenLane, Title: "Unresolved appeal", ReviewIntervalSeconds: &reviewSeconds, NextProbeAt: &nextProbe, RelativeRules: []services.NaturalLanguageRelativeRule{}},
	}}
	timezone, _ := models.NewTimezone(testsupport.TimezoneName)
	service, _ := services.NewNaturalLanguageService(fixture.Database, parser)
	before := temporalResourceCounts(testingContext, fixture.Database)
	draft, parseError := service.Parse(context.Background(), owner.ID, "unresolved appeal", calendarID, referenceTime, timezone)
	if parseError != nil {
		testingContext.Fatalf("parse waiting request: %v", parseError)
	}
	if draft.Status != models.IngestionDraftReady || draft.Source != models.IngestionSourceNatural || draft.Mode != models.IngestionModeOpenLane || draft.ReviewIntervalSeconds == nil || *draft.ReviewIntervalSeconds != reviewSeconds {
		testingContext.Fatalf("waiting draft = %#v", draft)
	}
	if got := temporalResourceCounts(testingContext, fixture.Database); got != before {
		testingContext.Fatalf("temporal counts before confirmation = %#v, want %#v", got, before)
	}
	if len(parser.requests) != 1 || !parser.requests[0].ReferenceTime.Equal(referenceTime) || parser.requests[0].Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("provider request = %#v", parser.requests)
	}
	confirmationService, _ := services.NewIngestionDraftService(fixture.Database, func() time.Time { return referenceTime })
	confirmation, _, confirmError := confirmationService.Confirm(context.Background(), owner.ID, draft.ID, "natural-waiting")
	if confirmError != nil || confirmation.AttentionPolicyID == nil {
		testingContext.Fatalf("waiting confirmation = %#v, error = %v", confirmation, confirmError)
	}
	var policy models.AttentionPolicy
	if findError := fixture.Database.First(&policy, "id = ?", *confirmation.AttentionPolicyID).Error; findError != nil || policy.ReviewIntervalSeconds != reviewSeconds {
		testingContext.Fatalf("waiting policy = %#v, error = %v", policy, findError)
	}
}

func TestNaturalLanguageFlightConfirmsAnchorAndRelativeRulesOnOneLane(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, seed)
	referenceTime := testsupport.FixedStartTime()
	start := referenceTime.Add(20 * 24 * time.Hour)
	end := start.Add(3 * time.Hour)
	startValue, endValue := start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)
	parser := &deterministicNaturalLanguageParser{responses: map[string]services.NaturalLanguageParseResponse{
		"flight": {Mode: models.IngestionModeDatedEvent, Title: "Flight", StartsAt: &startValue, EndsAt: &endValue, RelativeRules: []services.NaturalLanguageRelativeRule{{AnchorEdge: models.DerivedAnchorStart, OffsetSeconds: -7200}, {AnchorEdge: models.DerivedAnchorEnd, OffsetSeconds: 3600}}},
	}}
	timezone, _ := models.NewTimezone(testsupport.TimezoneName)
	service, _ := services.NewNaturalLanguageService(fixture.Database, parser)
	draft, parseError := service.Parse(context.Background(), owner.ID, "flight", calendarID, referenceTime, timezone)
	if parseError != nil {
		testingContext.Fatalf("parse flight: %v", parseError)
	}
	if len(draft.DerivedMarkerRules) != 2 {
		testingContext.Fatalf("draft rules = %#v", draft.DerivedMarkerRules)
	}
	var storedDraftRules []models.DraftDerivedMarkerRule
	if findError := fixture.Database.Where("draft_id = ?", draft.ID).Find(&storedDraftRules).Error; findError != nil || len(storedDraftRules) != 2 {
		testingContext.Fatalf("stored draft rules = %#v, error = %v", storedDraftRules, findError)
	}
	confirmationService, _ := services.NewIngestionDraftService(fixture.Database, func() time.Time { return referenceTime })
	confirmation, _, confirmError := confirmationService.Confirm(context.Background(), owner.ID, draft.ID, "natural-flight")
	if confirmError != nil || confirmation.EventID == nil {
		testingContext.Fatalf("flight confirmation = %#v, error = %v", confirmation, confirmError)
	}
	var rules []models.DerivedMarkerRule
	if findError := fixture.Database.Where("lane_id = ?", confirmation.LaneID).Order("offset_seconds ASC").Find(&rules).Error; findError != nil {
		testingContext.Fatalf("read derived rules: %v", findError)
	}
	if len(rules) != 2 || rules[0].AnchorID != *confirmation.EventID || rules[1].AnchorID != *confirmation.EventID {
		var allRules []models.DerivedMarkerRule
		fixture.Database.Find(&allRules)
		testingContext.Fatalf("confirmed rules = %#v, all rules = %#v", rules, allRules)
	}
	var markers []models.DerivedMarker
	if findError := fixture.Database.Where("lane_id = ?", confirmation.LaneID).Order("at ASC").Find(&markers).Error; findError != nil {
		testingContext.Fatalf("read derived markers: %v", findError)
	}
	if len(markers) != 2 || !markers[0].At.Equal(start.Add(-2*time.Hour)) || !markers[1].At.Equal(end.Add(time.Hour)) {
		testingContext.Fatalf("confirmed markers = %#v", markers)
	}
}

func TestNaturalLanguageInvalidResponseRollsBackAndIncompleteDraftCanBeCorrected(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, seed)
	referenceTime := testsupport.FixedStartTime()
	start := referenceTime.Add(24 * time.Hour)
	startValue := start.Format(time.RFC3339Nano)
	badTime := "tomorrow sometime"
	parser := &deterministicNaturalLanguageParser{responses: map[string]services.NaturalLanguageParseResponse{
		"invalid":    {Mode: models.IngestionModeDatedEvent, Title: "Invalid", StartsAt: &badTime},
		"incomplete": {Mode: models.IngestionModeDatedEvent, Title: "Incomplete flight", StartsAt: &startValue, RelativeRules: []services.NaturalLanguageRelativeRule{}},
	}}
	timezone, _ := models.NewTimezone(testsupport.TimezoneName)
	service, _ := services.NewNaturalLanguageService(fixture.Database, parser)
	var before int64
	fixture.Database.Model(&models.IngestionDraft{}).Count(&before)
	if _, parseError := service.Parse(context.Background(), owner.ID, "invalid", calendarID, referenceTime, timezone); !errors.Is(parseError, services.ErrNaturalLanguageProviderResponseInvalid) {
		testingContext.Fatalf("invalid provider error = %v", parseError)
	}
	var after int64
	fixture.Database.Model(&models.IngestionDraft{}).Count(&after)
	if after != before {
		testingContext.Fatalf("draft count after invalid response = %d, want %d", after, before)
	}
	draft, incompleteError := service.Parse(context.Background(), owner.ID, "incomplete", calendarID, referenceTime, timezone)
	if incompleteError != nil || draft.Status != models.IngestionDraftIncomplete {
		testingContext.Fatalf("incomplete draft = %#v, error = %v", draft, incompleteError)
	}
	missing, missingError := draft.MissingFields()
	if missingError != nil || len(missing) != 1 || missing[0] != "ends_at" {
		testingContext.Fatalf("missing fields = %#v, error = %v", missing, missingError)
	}
	confirmationService, _ := services.NewIngestionDraftService(fixture.Database, func() time.Time { return referenceTime })
	canceledDraft, secondParseError := service.Parse(context.Background(), owner.ID, "incomplete", calendarID, referenceTime, timezone)
	if secondParseError != nil {
		testingContext.Fatalf("parse cancelable incomplete draft: %v", secondParseError)
	}
	if canceled, cancelError := confirmationService.Cancel(context.Background(), owner.ID, canceledDraft.ID); cancelError != nil || canceled.Status != models.IngestionDraftCanceled {
		testingContext.Fatalf("cancel incomplete draft = %#v, error = %v", canceled, cancelError)
	}
	if _, _, confirmError := confirmationService.Confirm(context.Background(), owner.ID, draft.ID, "incomplete"); !errors.Is(confirmError, services.ErrIngestionDraftNotReady) {
		testingContext.Fatalf("incomplete confirmation error = %v", confirmError)
	}
	end := start.Add(2 * time.Hour)
	if _, updateError := confirmationService.Update(context.Background(), owner.ID, draft.ID, models.IngestionDraftProposal{Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: draft.Title, StartsAt: &start, EndsAt: &end, ReferenceTime: referenceTime, Timezone: testsupport.TimezoneName}); updateError != nil {
		testingContext.Fatalf("complete draft: %v", updateError)
	}
	if _, _, confirmError := confirmationService.Confirm(context.Background(), owner.ID, draft.ID, "completed"); confirmError != nil {
		testingContext.Fatalf("confirm completed draft: %v", confirmError)
	}
	columns, columnsError := fixture.Database.Migrator().ColumnTypes(&models.IngestionDraft{})
	if columnsError != nil {
		testingContext.Fatalf("read ingestion draft columns: %v", columnsError)
	}
	for _, column := range columns {
		if column.Name() == "input_text" {
			testingContext.Fatal("the canonical ingestion draft schema stores original input text")
		}
	}
}
