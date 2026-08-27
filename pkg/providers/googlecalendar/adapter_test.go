package googlecalendar_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/providers/googlecalendar"
)

func TestAdapterUsesReadOnlyConsentAndListsEveryCalendarPage(testingContext *testing.T) {
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if parseError := request.ParseForm(); parseError != nil {
				testingContext.Errorf("parse token form: %v", parseError)
			}
			if request.Form.Get("code") != "secret-code" || request.Form.Get("client_secret") != "client-secret" {
				testingContext.Errorf("token form is invalid")
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			fmt.Fprint(responseWriter, `{"access_token":"secret-access","refresh_token":"secret-refresh","expires_in":3600,"token_type":"Bearer"}`)
		case "/calendars":
			if request.Header.Get("Authorization") != "Bearer secret-access" {
				testingContext.Errorf("authorization header is invalid")
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			if request.URL.Query().Get("pageToken") == "next" {
				fmt.Fprint(responseWriter, `{"items":[{"id":"work","summary":"Work","timeZone":"America/Los_Angeles","backgroundColor":"#102030"}]}`)
				return
			}
			fmt.Fprint(responseWriter, `{"items":[{"id":"personal","summary":"Personal","timeZone":"America/Los_Angeles","backgroundColor":"#405060"}],"nextPageToken":"next"}`)
		default:
			http.NotFound(responseWriter, request)
		}
	}))
	defer server.Close()

	adapter, adapterError := googlecalendar.New(googlecalendar.Config{
		ClientID: "client-id", ClientSecret: "client-secret", AuthorizationEndpoint: server.URL + "/authorize",
		TokenEndpoint: server.URL + "/token", CalendarListEndpoint: server.URL + "/calendars",
	}, server.Client(), func() time.Time { return referenceTime })
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	authorizationURL, authorizationError := adapter.AuthorizationURL("state-value", "https://rsvp.example.test/calendar-connection-callbacks/google/")
	if authorizationError != nil {
		testingContext.Fatalf("create authorization URL: %v", authorizationError)
	}
	parsedAuthorizationURL, parseError := url.Parse(authorizationURL)
	if parseError != nil {
		testingContext.Fatalf("parse authorization URL: %v", parseError)
	}
	scopes := strings.Fields(parsedAuthorizationURL.Query().Get("scope"))
	if len(scopes) != 2 || scopes[0] != config.GoogleCalendarListReadonlyScope || scopes[1] != config.GoogleCalendarEventsReadonlyScope {
		testingContext.Fatalf("scopes = %#v", scopes)
	}
	if parsedAuthorizationURL.Query().Get("state") != "state-value" || parsedAuthorizationURL.Query().Get("access_type") != "offline" {
		testingContext.Fatalf("authorization query = %v", parsedAuthorizationURL.Query())
	}
	credential, exchangeError := adapter.ExchangeCode(context.Background(), "secret-code", "https://rsvp.example.test/calendar-connection-callbacks/google/")
	if exchangeError != nil {
		testingContext.Fatalf("exchange code: %v", exchangeError)
	}
	if credential.RefreshToken != "secret-refresh" || !credential.ExpiresAt.Equal(referenceTime.Add(time.Hour)) {
		testingContext.Fatalf("credential = %#v", credential)
	}
	calendars, listError := adapter.ListCalendars(context.Background(), credential)
	if listError != nil {
		testingContext.Fatalf("list calendars: %v", listError)
	}
	if len(calendars) != 2 || calendars[0].ID != "personal" || calendars[1].ID != "work" {
		testingContext.Fatalf("calendars = %#v", calendars)
	}
}

func TestAdapterProviderErrorDoesNotExposeResponseBody(testingContext *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(responseWriter, "secret-refresh")
	}))
	defer server.Close()
	adapter, adapterError := googlecalendar.New(googlecalendar.Config{ClientID: "id", ClientSecret: "secret", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, CalendarListEndpoint: server.URL}, server.Client(), time.Now)
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	_, exchangeError := adapter.ExchangeCode(context.Background(), "code", "https://rsvp.example.test/callback")
	if exchangeError == nil || strings.Contains(exchangeError.Error(), "secret-refresh") {
		testingContext.Fatalf("provider error = %v", exchangeError)
	}
}
