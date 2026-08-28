package googlecalendar_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/providers/googlecalendar"
	"github.com/tyemirov/RSVP/pkg/services"
)

func TestAdapterUsesReadOnlyConsentAndListsEveryCalendarPage(testingContext *testing.T) {
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if parseError := request.ParseForm(); parseError != nil {
				testingContext.Errorf("parse token form: %v", parseError)
			}
			if request.Form.Get("client_secret") != "client-secret" {
				testingContext.Errorf("token client secret is invalid")
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			switch request.Form.Get("grant_type") {
			case "authorization_code":
				if request.Form.Get("code") != "secret-code" {
					testingContext.Errorf("authorization code is invalid")
				}
				fmt.Fprint(responseWriter, `{"access_token":"secret-access","refresh_token":"secret-refresh","expires_in":3600,"token_type":"Bearer"}`)
			case "refresh_token":
				if request.Form.Get("refresh_token") != "secret-refresh" {
					testingContext.Errorf("refresh token is invalid")
				}
				fmt.Fprint(responseWriter, `{"access_token":"renewed-access","expires_in":7200,"token_type":"Bearer"}`)
			default:
				testingContext.Errorf("token grant type is invalid")
			}
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
		TokenEndpoint: server.URL + "/token", CalendarListEndpoint: server.URL + "/calendars", EventsEndpoint: server.URL + "/events",
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
	refreshed, refreshError := adapter.RefreshCredential(context.Background(), credential)
	if refreshError != nil {
		testingContext.Fatalf("refresh credential: %v", refreshError)
	}
	if refreshed.AccessToken != "renewed-access" || refreshed.RefreshToken != "secret-refresh" || !refreshed.ExpiresAt.Equal(referenceTime.Add(2*time.Hour)) {
		testingContext.Fatalf("refreshed credential = %#v", refreshed)
	}
}

func TestAdapterProviderErrorDoesNotExposeResponseBody(testingContext *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(responseWriter, "secret-refresh")
	}))
	defer server.Close()
	adapter, adapterError := googlecalendar.New(googlecalendar.Config{ClientID: "id", ClientSecret: "secret", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, CalendarListEndpoint: server.URL, EventsEndpoint: server.URL}, server.Client(), time.Now)
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	_, exchangeError := adapter.ExchangeCode(context.Background(), "code", "https://rsvp.example.test/callback")
	if exchangeError == nil || strings.Contains(exchangeError.Error(), "secret-refresh") {
		testingContext.Fatalf("provider error = %v", exchangeError)
	}
}

func TestAdapterPaginatesEventsAndReportsRejectedCursor(testingContext *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("syncToken") == "expired" {
			responseWriter.WriteHeader(http.StatusGone)
			return
		}
		if request.URL.Query().Get("singleEvents") != "true" || request.URL.Query().Get("showDeleted") != "true" {
			testingContext.Errorf("sync query = %v", request.URL.Query())
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("pageToken") == "page-2" {
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"occurrence","recurringEventId":"series","status":"confirmed","summary":"Review","start":{"dateTime":"2026-09-02T09:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-02T10:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"point","status":"confirmed","summary":"Deadline","start":{"dateTime":"2026-09-02T12:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-02T12:00:00-07:00","timeZone":"America/Los_Angeles"},"endTimeUnspecified":true},{"id":"same-day","status":"confirmed","summary":"One day","start":{"date":"2026-09-03"},"end":{"date":"2026-09-03"}}],"nextSyncToken":"cursor-1"}`)
			return
		}
		fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"birthday","status":"confirmed","summary":"Birthday","start":{"date":"2026-09-01"},"end":{"date":"2026-09-02"}}],"nextPageToken":"page-2"}`)
	}))
	defer server.Close()
	adapter, adapterError := googlecalendar.New(googlecalendar.Config{ClientID: "id", ClientSecret: "secret", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, CalendarListEndpoint: server.URL, EventsEndpoint: server.URL}, server.Client(), time.Now)
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	credential := services.CalendarProviderCredential{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}
	batch, syncError := adapter.SynchronizeEvents(context.Background(), credential, "source/calendar", "")
	if syncError != nil {
		testingContext.Fatalf("synchronize events: %v", syncError)
	}
	if len(batch.Events) != 4 || batch.NextSyncCursor != "cursor-1" || batch.Events[0].StartDate != "2026-09-01" || batch.Events[1].SeriesID != "series" || batch.Events[2].At == nil || batch.Events[2].StartsAt != nil || batch.Events[2].EndsAt != nil || batch.Events[3].StartDate != "2026-09-03" || batch.Events[3].EndDate != "2026-09-04" {
		testingContext.Fatalf("batch = %#v", batch)
	}
	if _, rejectedError := adapter.SynchronizeEvents(context.Background(), credential, "source/calendar", "expired"); !errors.Is(rejectedError, services.ErrCalendarSyncCursorRejected) {
		testingContext.Fatalf("rejected cursor error = %v", rejectedError)
	}
}
