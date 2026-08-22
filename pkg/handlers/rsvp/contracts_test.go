package rsvp_test

import (
	"bytes"
	"encoding/base64"
	"html"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/liyue201/goqr"
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
	if decodedPayload := decodeQRCodePayload(testingContext, responseBody); decodedPayload != expectedPublicURL {
		testingContext.Fatalf("QR payload = %q, want %q", decodedPayload, expectedPublicURL)
	}
}

func decodeQRCodePayload(testingContext *testing.T, responseBody string) string {
	testingContext.Helper()
	const qrImagePrefix = `<img src="data:image/png;base64,`
	imageStart := strings.Index(responseBody, qrImagePrefix)
	if imageStart == -1 {
		testingContext.Fatal("QR page does not contain an encoded PNG image")
	}
	encodedImageStart := imageStart + len(qrImagePrefix)
	encodedImageEnd := strings.Index(responseBody[encodedImageStart:], `"`)
	if encodedImageEnd == -1 {
		testingContext.Fatal("QR image data does not have a closing quote")
	}
	encodedImage := responseBody[encodedImageStart : encodedImageStart+encodedImageEnd]
	encodedImage, unescapeError := url.PathUnescape(html.UnescapeString(encodedImage))
	if unescapeError != nil {
		testingContext.Fatalf("unescape QR image data: %v", unescapeError)
	}
	imageBytes, decodeError := base64.StdEncoding.DecodeString(encodedImage)
	if decodeError != nil {
		testingContext.Fatalf("decode QR image data: %v", decodeError)
	}
	qrImage, pngError := png.Decode(bytes.NewReader(imageBytes))
	if pngError != nil {
		testingContext.Fatalf("decode QR PNG: %v", pngError)
	}
	qrCodes, recognitionError := goqr.Recognize(qrImage)
	if recognitionError != nil {
		testingContext.Fatalf("recognize QR code: %v", recognitionError)
	}
	if len(qrCodes) != 1 {
		testingContext.Fatalf("recognized QR code count = %d, want 1", len(qrCodes))
	}
	return string(qrCodes[0].Payload)
}
