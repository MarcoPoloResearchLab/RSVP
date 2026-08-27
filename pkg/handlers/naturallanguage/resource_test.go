package naturallanguage

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

func TestProviderFailureLogDoesNotContainInputText(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	var lane models.Lane
	if findError := fixture.Database.First(&lane, "id = ?", seed.LaneID).Error; findError != nil {
		testingContext.Fatalf("read fixture lane: %v", findError)
	}
	var logOutput bytes.Buffer
	fixture.ApplicationContext.Logger = log.New(&logOutput, "", 0)
	resource, resourceError := New(fixture.ApplicationContext, failingParser{})
	if resourceError != nil {
		testingContext.Fatalf("construct natural-language resource: %v", resourceError)
	}
	privateInput := "private appeal details"
	body := `{"input_text":"` + privateInput + `","calendar_id":"` + lane.CalendarID + `","reference_time":"` + testsupport.FixedStartTime().Format(time.RFC3339Nano) + `","timezone":"` + testsupport.TimezoneName + `"}`
	request := httptest.NewRequest(http.MethodPost, config.WebNaturalLanguageIngestion, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	responseRecorder := httptest.NewRecorder()

	resource.Handler().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusBadGateway, responseRecorder.Body.String())
	}
	if strings.Contains(logOutput.String(), privateInput) || strings.Contains(responseRecorder.Body.String(), privateInput) {
		testingContext.Fatalf("provider failure leaked input: log = %q, body = %q", logOutput.String(), responseRecorder.Body.String())
	}
}
