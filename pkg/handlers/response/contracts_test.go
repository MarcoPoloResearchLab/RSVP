package response_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/response"
	"github.com/tyemirov/RSVP/pkg/routes"
)

func TestPublicResponseRouteShowsInvitation(testingContext *testing.T) {
	testsupport.LoadTemplates(testingContext)
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, &venueRecord.ID)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	request := testsupport.Request(
		testingContext,
		http.MethodGet,
		config.WebResponse+"?"+config.RSVPIDParam+"="+url.QueryEscape(rsvpRecord.ID),
		nil,
		nil,
	)
	responseRecorder := httptest.NewRecorder()

	response.Handler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	responseBody := responseRecorder.Body.String()
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
	request := testsupport.Request(
		testingContext,
		http.MethodPost,
		config.WebResponse+"?"+config.RSVPIDParam+"="+url.QueryEscape(rsvpRecord.ID),
		formValues,
		nil,
	)
	responseRecorder := httptest.NewRecorder()
	routesInstance := routes.New(fixture.ApplicationContext, config.EnvConfig{})
	publicHandler := routesInstance.ApplyOverrides(response.Handler(fixture.ApplicationContext))

	publicHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}
	expectedLocation := config.WebResponseThankYou + "?" + config.RSVPIDParam + "=" + rsvpRecord.ID
	if actualLocation := responseRecorder.Header().Get("Location"); actualLocation != expectedLocation {
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
