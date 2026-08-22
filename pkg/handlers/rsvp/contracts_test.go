package rsvp_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/rsvp"
)

func TestRSVPOwnershipRejectsOtherOwner(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	formValues := url.Values{
		config.NameParam:        {"Changed Invitee"},
		config.ResponseParam:    {config.RSVPResponseYesPrefix},
		config.ExtraGuestsParam: {"1"},
	}
	request := testsupport.Request(
		testingContext,
		http.MethodPut,
		config.WebRSVPs+"?"+config.RSVPIDParam+"="+url.QueryEscape(rsvpRecord.ID),
		formValues,
		&otherOwner,
	)
	responseRecorder := httptest.NewRecorder()

	rsvp.UpdateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusForbidden, responseRecorder.Body.String())
	}
	var storedRSVP models.RSVP
	if findError := fixture.Database.First(&storedRSVP, "id = ?", rsvpRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find RSVP after rejected update: %v", findError)
	}
	if storedRSVP.Name != rsvpRecord.Name {
		testingContext.Fatalf("RSVP name = %q, want %q", storedRSVP.Name, rsvpRecord.Name)
	}
	if storedRSVP.Response != rsvpRecord.Response {
		testingContext.Fatalf("RSVP response = %q, want %q", storedRSVP.Response, rsvpRecord.Response)
	}
	if storedRSVP.ExtraGuests != rsvpRecord.ExtraGuests {
		testingContext.Fatalf("RSVP extra guests = %d, want %d", storedRSVP.ExtraGuests, rsvpRecord.ExtraGuests)
	}
}

func TestQRCodePageUsesCanonicalPublicResponseURL(testingContext *testing.T) {
	testsupport.LoadTemplates(testingContext)
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	request := testsupport.Request(
		testingContext,
		http.MethodGet,
		config.WebRSVPQR+"?"+config.RSVPIDParam+"="+url.QueryEscape(rsvpRecord.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	rsvp.ShowHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	expectedPublicURL := testsupport.ApplicationBaseURL + "response/?" + config.RSVPIDParam + "=" + rsvpRecord.ID
	responseBody := responseRecorder.Body.String()
	if !strings.Contains(responseBody, expectedPublicURL) {
		testingContext.Fatalf("QR page does not contain public URL %q", expectedPublicURL)
	}
	if !strings.Contains(responseBody, "data:image/png;base64,") {
		testingContext.Fatal("QR page does not contain an encoded PNG image")
	}
}
