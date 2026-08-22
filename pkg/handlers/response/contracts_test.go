package response_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
)

func TestPublicResponseRouteShowsInvitation(testingContext *testing.T) {
	testsupport.LoadTemplates(testingContext)
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, &venueRecord.ID)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	publicServer := newPublicResponseServer(testingContext, fixture)
	requestURL := publicServer.URL + config.WebResponse + "?" + config.RSVPIDParam + "=" + url.QueryEscape(rsvpRecord.ID)

	httpResponse, requestError := publicServer.Client().Get(requestURL)
	if requestError != nil {
		testingContext.Fatalf("get public response route: %v", requestError)
	}
	responseBody := readResponseBody(testingContext, httpResponse)

	if httpResponse.StatusCode != http.StatusOK {
		testingContext.Fatalf("status = %d, want %d; body = %s", httpResponse.StatusCode, http.StatusOK, responseBody)
	}
	expectedContent := []string{
		"You're Invited!",
		eventRecord.Title,
		rsvpRecord.Name,
		venueRecord.Name,
		"name=\"_method\" value=\"PUT\"",
	}
	for _, expectedSubstring := range expectedContent {
		if !strings.Contains(responseBody, expectedSubstring) {
			testingContext.Errorf("response page does not contain %q", expectedSubstring)
		}
	}
}

func TestPublicResponseRouteUpdatesRSVP(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	formValues := url.Values{
		config.MethodOverrideParam: {http.MethodPut},
		config.ResponseParam:       {config.RSVPResponseYesPrefix},
		config.ExtraGuestsParam:    {"2"},
	}
	publicServer := newPublicResponseServer(testingContext, fixture)
	requestURL := publicServer.URL + config.WebResponse + "?" + config.RSVPIDParam + "=" + url.QueryEscape(rsvpRecord.ID)
	httpClient := publicServer.Client()
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	httpResponse, requestError := httpClient.PostForm(requestURL, formValues)
	if requestError != nil {
		testingContext.Fatalf("update RSVP through public response route: %v", requestError)
	}
	responseBody := readResponseBody(testingContext, httpResponse)

	if httpResponse.StatusCode != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", httpResponse.StatusCode, http.StatusSeeOther, responseBody)
	}
	expectedLocation := config.WebResponseThankYou + "?" + config.RSVPIDParam + "=" + rsvpRecord.ID
	if actualLocation := httpResponse.Header.Get("Location"); actualLocation != expectedLocation {
		testingContext.Fatalf("redirect location = %q, want %q", actualLocation, expectedLocation)
	}

	var storedRSVP models.RSVP
	if findError := fixture.Database.First(&storedRSVP, "id = ?", rsvpRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find updated RSVP: %v", findError)
	}
	if storedRSVP.Response != config.RSVPResponseYesPrefix {
		testingContext.Fatalf("RSVP response = %q, want %q", storedRSVP.Response, config.RSVPResponseYesPrefix)
	}
	if storedRSVP.ExtraGuests != 2 {
		testingContext.Fatalf("RSVP extra guests = %d, want 2", storedRSVP.ExtraGuests)
	}
}

func newPublicResponseServer(testingContext *testing.T, fixture *testsupport.Fixture) *httptest.Server {
	testingContext.Helper()
	mux := http.NewServeMux()
	routes.New(fixture.ApplicationContext, config.EnvConfig{}).RegisterRoutes(mux)
	publicServer := httptest.NewServer(mux)
	testingContext.Cleanup(publicServer.Close)
	return publicServer
}

func readResponseBody(testingContext *testing.T, httpResponse *http.Response) string {
	testingContext.Helper()
	responseBody, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		testingContext.Fatalf("read HTTP response body: %v", readError)
	}
	if closeError := httpResponse.Body.Close(); closeError != nil {
		testingContext.Fatalf("close HTTP response body: %v", closeError)
	}
	return string(responseBody)
}
