package ingestiondraft

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
)

type failingParser struct{}

func (failingParser) Parse(_ context.Context, request services.NaturalLanguageParseRequest) (services.NaturalLanguageParseResponse, error) {
	return services.NaturalLanguageParseResponse{}, errors.New("provider rejected " + request.InputText)
}

type successfulParser struct{}

func (successfulParser) Parse(_ context.Context, _ services.NaturalLanguageParseRequest) (services.NaturalLanguageParseResponse, error) {
	return services.NaturalLanguageParseResponse{Mode: models.IngestionModeOpenLane, Title: "Parsed waiting item"}, nil
}

func TestDraftRequestInterpretsLocalTimesInProposedTimezone(testingContext *testing.T) {
	localStart := "2030-02-04T10:30"
	localEnd := "2030-02-04T12:00"
	request := draftRequest{
		Mode: models.IngestionModeDatedEvent, CalendarID: "CAL00001", Title: "Timezone correction",
		StartsAt: &localStart, EndsAt: &localEnd, ReferenceTime: "2030-01-02T15:00:00Z", Timezone: "America/New_York",
	}
	proposal, proposalError := request.proposal()
	if proposalError != nil {
		testingContext.Fatalf("construct proposal: %v", proposalError)
	}
	wantStart := time.Date(2030, time.February, 4, 15, 30, 0, 0, time.UTC)
	wantEnd := time.Date(2030, time.February, 4, 17, 0, 0, 0, time.UTC)
	if proposal.StartsAt == nil || !proposal.StartsAt.Equal(wantStart) || proposal.EndsAt == nil || !proposal.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("proposal times = %v to %v, want %v to %v", proposal.StartsAt, proposal.EndsAt, wantStart, wantEnd)
	}
}

func TestNaturalLanguageDraftUsesCanonicalCollectionWithoutLeakingInput(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	var lane models.Lane
	if findError := fixture.Database.First(&lane, "id = ?", seed.LaneID).Error; findError != nil {
		testingContext.Fatalf("read fixture lane: %v", findError)
	}
	var logOutput bytes.Buffer
	fixture.ApplicationContext.Logger = log.New(&logOutput, "", 0)
	resources, resourceError := New(fixture.ApplicationContext, time.Now, failingParser{})
	if resourceError != nil {
		testingContext.Fatalf("construct ingestion-draft resources: %v", resourceError)
	}
	privateInput := "private appeal details"
	body := `{"source":"natural_language","input_text":"` + privateInput + `","calendar_id":"` + lane.CalendarID + `","reference_time":"` + testsupport.FixedStartTime().Format(time.RFC3339Nano) + `","timezone":"` + testsupport.TimezoneName + `"}`
	request := httptest.NewRequest(http.MethodPost, config.WebIngestionDrafts, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	responseRecorder := httptest.NewRecorder()

	resources.Handler().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusBadGateway, responseRecorder.Body.String())
	}
	if strings.Contains(logOutput.String(), privateInput) || strings.Contains(responseRecorder.Body.String(), privateInput) {
		testingContext.Fatalf("provider failure leaked input: log = %q, body = %q", logOutput.String(), responseRecorder.Body.String())
	}
}

func TestCanonicalCollectionCreatesQuickAndNaturalLanguageDrafts(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	var lane models.Lane
	if findError := fixture.Database.First(&lane, "id = ?", seed.LaneID).Error; findError != nil {
		testingContext.Fatalf("read fixture lane: %v", findError)
	}
	resources, resourceError := New(fixture.ApplicationContext, time.Now, successfulParser{})
	if resourceError != nil {
		testingContext.Fatalf("construct ingestion-draft resources: %v", resourceError)
	}
	requestDraft := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, config.WebIngestionDrafts, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
		responseRecorder := httptest.NewRecorder()
		resources.Handler().ServeHTTP(responseRecorder, request)
		return responseRecorder
	}
	referenceTime := testsupport.FixedStartTime().Format(time.RFC3339Nano)
	quickStart := testsupport.FixedStartTime().Add(24 * time.Hour).Format(time.RFC3339Nano)
	quickEnd := testsupport.FixedStartTime().Add(25 * time.Hour).Format(time.RFC3339Nano)
	quickResponse := requestDraft(`{"source":"quick","mode":"dated_event","calendar_id":"` + lane.CalendarID + `","title":"Quick draft","starts_at":"` + quickStart + `","ends_at":"` + quickEnd + `","reference_time":"` + referenceTime + `","timezone":"` + testsupport.TimezoneName + `"}`)
	naturalResponse := requestDraft(`{"source":"natural_language","input_text":"wait for a decision","calendar_id":"` + lane.CalendarID + `","reference_time":"` + referenceTime + `","timezone":"` + testsupport.TimezoneName + `"}`)

	for responseName, responseRecorder := range map[string]*httptest.ResponseRecorder{"quick": quickResponse, "natural": naturalResponse} {
		if responseRecorder.Code != http.StatusCreated || !strings.HasPrefix(responseRecorder.Header().Get("Location"), config.WebIngestionDrafts) {
			testingContext.Fatalf("%s response = %d Location %q; body = %s", responseName, responseRecorder.Code, responseRecorder.Header().Get("Location"), responseRecorder.Body.String())
		}
	}
	var drafts []models.IngestionDraft
	if findError := fixture.Database.Order("created_at ASC").Find(&drafts).Error; findError != nil {
		testingContext.Fatalf("read drafts: %v", findError)
	}
	if len(drafts) != 2 || drafts[0].Source != models.IngestionSourceQuick || drafts[1].Source != models.IngestionSourceNatural {
		testingContext.Fatalf("draft sources = %#v", drafts)
	}
}
