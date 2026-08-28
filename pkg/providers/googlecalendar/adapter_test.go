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
			if request.URL.Query().Get("syncToken") == "expired" {
				responseWriter.WriteHeader(http.StatusGone)
				return
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			if request.URL.Query().Get("syncToken") == "calendar-cursor-1" {
				if request.URL.Query().Get("showHidden") != "true" {
					testingContext.Errorf("incremental CalendarList query = %v", request.URL.Query())
				}
				fmt.Fprint(responseWriter, `{"items":[{"id":"personal","deleted":true},{"id":"family","summary":"Family","timeZone":"America/Los_Angeles","backgroundColor":"#708090","selected":true,"accessRole":"reader"}],"nextSyncToken":"calendar-cursor-2"}`)
				return
			}
			if request.URL.Query().Get("showHidden") != "true" {
				testingContext.Errorf("complete CalendarList query = %v", request.URL.Query())
			}
			if request.URL.Query().Get("pageToken") == "next" {
				fmt.Fprint(responseWriter, `{"items":[{"id":"work","summary":"Work","summaryOverride":"Team","timeZone":"America/Los_Angeles","backgroundColor":"#102030","accessRole":"owner"}],"nextSyncToken":"calendar-cursor-1"}`)
				return
			}
			fmt.Fprint(responseWriter, `{"items":[{"id":"personal","summary":"Personal","timeZone":"America/Los_Angeles","backgroundColor":"#405060","selected":true,"accessRole":"reader","primary":true},{"id":"addressbook#contacts@group.v.calendar.google.com","summary":"Contacts birthdays","timeZone":"America/Los_Angeles","backgroundColor":"#778899","selected":true,"accessRole":"reader"}],"nextPageToken":"next"}`)
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
	calendarBatch, listError := adapter.ListCalendars(context.Background(), credential, "")
	if listError != nil {
		testingContext.Fatalf("list calendars: %v", listError)
	}
	if len(calendarBatch.Calendars) != 3 || calendarBatch.NextSyncCursor != "calendar-cursor-1" || calendarBatch.Calendars[0].ID != "personal" || !calendarBatch.Calendars[0].Readable || len(calendarBatch.Calendars[0].Groups) != 2 || calendarBatch.Calendars[0].Groups[0].Name != "Personal" || calendarBatch.Calendars[0].Groups[1].Key != services.ProviderCalendarGroupBirthdays || calendarBatch.Calendars[0].Groups[1].Name != "Birthdays" || calendarBatch.Calendars[1].Readable || len(calendarBatch.Calendars[1].Groups) != 0 || calendarBatch.Calendars[2].Groups[0].Name != "Team" || calendarBatch.Calendars[2].Groups[0].Visible {
		testingContext.Fatalf("calendar batch = %#v", calendarBatch)
	}
	incrementalBatch, incrementalError := adapter.ListCalendars(context.Background(), credential, "calendar-cursor-1")
	if incrementalError != nil || len(incrementalBatch.Calendars) != 2 || !incrementalBatch.Calendars[0].Deleted || incrementalBatch.Calendars[1].Groups[0].Name != "Family" || incrementalBatch.NextSyncCursor != "calendar-cursor-2" {
		testingContext.Fatalf("incremental calendar batch = %#v, error = %v", incrementalBatch, incrementalError)
	}
	if _, rejectedError := adapter.ListCalendars(context.Background(), credential, "expired"); !errors.Is(rejectedError, services.ErrCalendarListSyncCursorRejected) {
		testingContext.Fatalf("rejected CalendarList cursor error = %v", rejectedError)
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
		if request.URL.Query().Has("eventTypes") {
			testingContext.Errorf("grouped event query = %v", request.URL.Query())
		}
		if request.URL.Query().Get("pageToken") == "page-2" {
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"occurrence","eventType":"default","recurringEventId":"series","status":"confirmed","summary":"Review","start":{"dateTime":"2026-09-02T09:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-02T10:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"point","eventType":"default","status":"confirmed","summary":"Deadline","start":{"dateTime":"2026-09-02T12:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-02T12:00:00-07:00","timeZone":"America/Los_Angeles"},"endTimeUnspecified":true},{"id":"same-day","eventType":"default","status":"confirmed","summary":"One day","start":{"date":"2026-09-03"},"end":{"date":"2026-09-03"}},{"id":"unknown-type","eventType":"providerFutureType","status":"confirmed","summary":"Provider future event","start":{"date":"2026-09-04"},"end":{"date":"2026-09-05"}},{"id":"unbirthday","eventType":"default","status":"confirmed","summary":"Unbirthday planning","start":{"date":"2026-09-05"},"end":{"date":"2026-09-06"}},{"id":"birthday-by-type","eventType":"birthday","status":"confirmed","summary":"Annual contact","start":{"date":"2026-09-06"},"end":{"date":"2026-09-07"}},{"id":"birthday-by-properties","birthdayProperties":{},"status":"confirmed","summary":"Annual friend reminder","start":{"date":"2026-09-07"},"end":{"date":"2026-09-08"}},{"id":"birthday-by-title","eventType":"default","status":"confirmed","summary":"Andrew Fert Birthday","start":{"date":"2026-09-08"},"end":{"date":"2026-09-09"}}],"nextSyncToken":"cursor-1"}`)
			return
		}
		fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[],"nextPageToken":"page-2"}`)
	}))
	defer server.Close()
	adapter, adapterError := googlecalendar.New(googlecalendar.Config{ClientID: "id", ClientSecret: "secret", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, CalendarListEndpoint: server.URL, EventsEndpoint: server.URL}, server.Client(), time.Now)
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	credential := services.CalendarProviderCredential{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}
	batch, syncError := adapter.SynchronizeEvents(context.Background(), credential, "source/calendar", services.ProviderCalendarGroupCalendar, "")
	if syncError != nil {
		testingContext.Fatalf("synchronize events: %v", syncError)
	}
	if len(batch.Events) != 8 || batch.NextSyncCursor != "cursor-1" || batch.Events[0].SeriesID != "series" || batch.Events[1].At == nil || batch.Events[1].StartsAt != nil || batch.Events[1].EndsAt != nil || batch.Events[2].StartDate != "2026-09-03" || batch.Events[2].EndDate != "2026-09-04" || batch.Events[3].Title != "Provider future event" || batch.Events[4].Title != "Unbirthday planning" || batch.Events[5].Status != "cancelled" || batch.Events[7].Status != "cancelled" {
		testingContext.Fatalf("batch = %#v", batch)
	}
	birthdayBatch, birthdayError := adapter.SynchronizeEvents(context.Background(), credential, "source/calendar", services.ProviderCalendarGroupBirthdays, "")
	if birthdayError != nil || len(birthdayBatch.Events) != 8 || birthdayBatch.Events[0].Status != "cancelled" || birthdayBatch.Events[4].Status != "cancelled" || birthdayBatch.Events[5].Title != "Annual contact" || birthdayBatch.Events[7].Title != "Andrew Fert Birthday" || birthdayBatch.NextSyncCursor != "cursor-1" {
		testingContext.Fatalf("birthday batch = %#v, error = %v", birthdayBatch, birthdayError)
	}
	if _, rejectedError := adapter.SynchronizeEvents(context.Background(), credential, "source/calendar", services.ProviderCalendarGroupCalendar, "expired"); !errors.Is(rejectedError, services.ErrCalendarSyncCursorRejected) {
		testingContext.Fatalf("rejected cursor error = %v", rejectedError)
	}
}
