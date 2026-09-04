// Command browserfixture serves deterministic horizon data for real-browser tests.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaussConstants "github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/templates"
	"gorm.io/gorm"
)

const (
	browserFixtureAddress      = "127.0.0.1:18080"
	browserOrganizerID         = "USRBRWSR"
	browserOrganizerEmail      = "horizon@example.test"
	browserNewOrganizerEmail   = "new-horizon@example.test"
	browserTimezoneName        = "America/Los_Angeles"
	browserLoginPath           = "/browser-login/"
	browserNewLoginPath        = "/browser-new-login/"
	browserGoogleAuthorizePath = "/browser-google/authorize"
	browserGoogleTokenPath     = "/browser-google/token"
	browserGoogleCalendarsPath = "/browser-google/calendars"
	browserGoogleEventsPath    = "/browser-google/events"
	browserNaturalLanguagePath = "/browser-natural-language"
)

var browserReferenceTime = time.Now().UTC()

func main() {
	logger := log.New(os.Stdout, "browserfixture: ", log.LstdFlags)
	databasePath := filepath.Join("output", "playwright", "horizon-browser.db")
	if directoryError := os.MkdirAll(filepath.Dir(databasePath), 0755); directoryError != nil {
		logger.Fatalf("Create browser fixture directory: %v", directoryError)
	}
	if removeError := os.Remove(databasePath); removeError != nil && !os.IsNotExist(removeError) {
		logger.Fatalf("Remove prior browser fixture database: %v", removeError)
	}
	database, databaseError := services.OpenDatabase(databasePath)
	if databaseError != nil {
		logger.Fatalf("Open browser fixture database: %v", databaseError)
	}
	if seedError := seedBrowserFixture(database); seedError != nil {
		logger.Fatalf("Seed browser fixture: %v", seedError)
	}
	templates.LoadAllPrecompiledTemplates(config.TemplatesDir)
	session.NewSession([]byte("0123456789abcdef0123456789abcdef"))

	applicationContext := &config.ApplicationContext{Database: database, Logger: logger, AppBaseURL: "http://" + browserFixtureAddress + "/"}
	mux := http.NewServeMux()
	mux.HandleFunc(browserLoginPath, func(responseWriter http.ResponseWriter, request *http.Request) {
		setBrowserSession(responseWriter, request, browserOrganizerEmail, "Horizon Browser")
	})
	mux.HandleFunc(browserNewLoginPath, func(responseWriter http.ResponseWriter, request *http.Request) {
		setBrowserSession(responseWriter, request, browserNewOrganizerEmail, "New Horizon Browser")
	})
	mux.HandleFunc(browserGoogleAuthorizePath, func(responseWriter http.ResponseWriter, request *http.Request) {
		redirectURI := request.URL.Query().Get("redirect_uri")
		state := request.URL.Query().Get("state")
		if redirectURI == "" || state == "" {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		callbackURL, parseError := url.Parse(redirectURI)
		if parseError != nil {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		query := callbackURL.Query()
		query.Set("state", state)
		query.Set("code", "browser-authorization-code")
		callbackURL.RawQuery = query.Encode()
		http.Redirect(responseWriter, request, callbackURL.String(), http.StatusFound)
	})
	mux.HandleFunc(browserGoogleTokenPath, func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(responseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		fmt.Fprint(responseWriter, `{"access_token":"browser-access","refresh_token":"browser-refresh","expires_in":3600,"token_type":"Bearer"}`)
	})
	mux.HandleFunc(browserGoogleCalendarsPath, func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer browser-access" {
			http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("syncToken") != "" {
			fmt.Fprint(responseWriter, `{"items":[],"nextSyncToken":"browser-calendar-list-2"}`)
			return
		}
		if request.URL.Query().Get("showHidden") != "true" {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		fmt.Fprint(responseWriter, `{"items":[{"id":"google-primary","summary":"temirov@gmail.com","timeZone":"America/Los_Angeles","backgroundColor":"#9a9cff","selected":true,"accessRole":"owner","primary":true},{"id":"google-holidays","summary":"Holidays","timeZone":"America/Los_Angeles","backgroundColor":"#42d692","selected":true,"accessRole":"reader"},{"id":"google-family","summary":"Family","timeZone":"America/Los_Angeles","backgroundColor":"#9fc6e7","selected":false,"accessRole":"reader"},{"id":"addressbook#contacts@group.v.calendar.google.com","summary":"Contacts birthdays","timeZone":"America/Los_Angeles","backgroundColor":"#778899","selected":true,"accessRole":"reader"}],"nextSyncToken":"browser-calendar-list-1"}`)
	})
	mux.HandleFunc(browserGoogleEventsPath+"/", func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer browser-access" || request.URL.Query().Get("singleEvents") != "true" || request.URL.Query().Get("showDeleted") != "true" {
			http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/google-holidays/events") {
			if request.URL.Query().Get("syncToken") != "" {
				fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[],"nextSyncToken":"browser-holidays-sync-2"}`)
				return
			}
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"holiday-founders","status":"confirmed","summary":"Provider Founders Day","start":{"date":"2026-09-20"},"end":{"date":"2026-09-21"}},{"id":"holiday-autumn","status":"confirmed","summary":"Provider Autumn Holiday","start":{"date":"2026-09-24"},"end":{"date":"2026-09-25"}}],"nextSyncToken":"browser-holidays-sync-1"}`)
			return
		}
		if strings.HasSuffix(request.URL.Path, "/google-family/events") {
			if request.URL.Query().Get("syncToken") != "" {
				fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[],"nextSyncToken":"browser-family-sync-2"}`)
				return
			}
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"family-dinner-1","recurringEventId":"family-dinner","status":"confirmed","summary":"Provider family dinner","start":{"dateTime":"2026-09-18T18:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-18T19:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"family-dinner-2","recurringEventId":"family-dinner","status":"confirmed","summary":"Provider family dinner","start":{"dateTime":"2026-09-25T18:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-25T19:00:00-07:00","timeZone":"America/Los_Angeles"}}],"nextSyncToken":"browser-family-sync-1"}`)
			return
		}
		if strings.HasSuffix(request.URL.Path, "/addressbook#contacts@group.v.calendar.google.com/events") {
			if request.URL.Query().Get("syncToken") != "" {
				fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[],"nextSyncToken":"browser-contacts-sync-2"}`)
				return
			}
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"contact-birthday","eventType":"birthday","birthdayProperties":{"type":"birthday","contact":"people/100"},"status":"confirmed","summary":"Happy birthday!","start":{"date":"2026-09-22"},"end":{"date":"2026-09-23"}},{"id":"contact-anniversary","eventType":"birthday","birthdayProperties":{"type":"anniversary","contact":"people/101"},"status":"confirmed","summary":"Contact anniversary","start":{"date":"2026-09-23"},"end":{"date":"2026-09-24"}}],"nextSyncToken":"browser-contacts-sync-1"}`)
			return
		}
		if !strings.HasSuffix(request.URL.Path, "/google-primary/events") {
			http.Error(responseWriter, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if request.URL.Query().Get("syncToken") != "" {
			fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"primary-review","eventType":"default","status":"confirmed","summary":"Primary review birthday","start":{"dateTime":"2026-09-10T09:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-10T10:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"birthday-ada","eventType":"default","status":"confirmed","summary":"Ada provider birthday updated","start":{"date":"2026-09-15"},"end":{"date":"2026-09-16"}},{"id":"birthday-lin","status":"cancelled"},{"id":"birthday-maya","status":"cancelled"}],"nextSyncToken":"browser-primary-sync-2"}`)
			return
		}
		time.Sleep(2500 * time.Millisecond)
		fmt.Fprint(responseWriter, `{"timeZone":"America/Los_Angeles","items":[{"id":"primary-review","eventType":"default","status":"confirmed","summary":"Primary review","start":{"dateTime":"2026-09-10T09:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-10T10:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"primary-future-type","eventType":"providerFutureType","status":"confirmed","summary":"Provider future event","start":{"dateTime":"2026-09-11T09:00:00-07:00","timeZone":"America/Los_Angeles"},"end":{"dateTime":"2026-09-11T10:00:00-07:00","timeZone":"America/Los_Angeles"}},{"id":"birthday-ada","eventType":"default","status":"confirmed","summary":"Ada provider birthday","start":{"date":"2026-09-15"},"end":{"date":"2026-09-16"}},{"id":"birthday-lin","eventType":"birthday","status":"confirmed","summary":"Lin provider birthday","start":{"date":"2026-10-01"},"end":{"date":"2026-10-02"}},{"id":"birthday-maya","eventType":"birthday","birthdayProperties":{"type":"self"},"status":"confirmed","summary":"Maya provider birthday","start":{"date":"2026-10-20"},"end":{"date":"2026-10-21"}}],"nextSyncToken":"browser-primary-sync-1"}`)
	})
	mux.HandleFunc(browserNaturalLanguagePath, func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer browser-parser-key" {
			http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		var body struct {
			InputText string `json:"input_text"`
		}
		if decodeError := json.NewDecoder(request.Body).Decode(&body); decodeError != nil {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		switch body.InputText {
		case "unresolved appeal with weekly checks":
			nextProbe := browserReferenceTime.Add(7 * 24 * time.Hour).Format(time.RFC3339Nano)
			fmt.Fprintf(responseWriter, `{"mode":"open_lane","title":"Unresolved appeal","anchor_event_id":null,"starts_at":null,"ends_at":null,"review_interval_seconds":604800,"next_probe_at":%q,"escalation_interval_seconds":null,"relative_rules":[]}`, nextProbe)
		case "flight with relative departure and arrival markers":
			startsAt := browserReferenceTime.Add(20 * 24 * time.Hour).Format(time.RFC3339Nano)
			endsAt := browserReferenceTime.Add(20*24*time.Hour + 3*time.Hour).Format(time.RFC3339Nano)
			fmt.Fprintf(responseWriter, `{"mode":"dated_event","title":"Parsed flight","anchor_event_id":null,"starts_at":%q,"ends_at":%q,"review_interval_seconds":null,"next_probe_at":null,"escalation_interval_seconds":null,"relative_rules":[{"anchor_edge":"start","offset_seconds":-7200},{"anchor_edge":"end","offset_seconds":3600}]}`, startsAt, endsAt)
		case "flight missing an end":
			startsAt := browserReferenceTime.Add(24 * 24 * time.Hour).Format(time.RFC3339Nano)
			fmt.Fprintf(responseWriter, `{"mode":"dated_event","title":"Incomplete flight","anchor_event_id":null,"starts_at":%q,"ends_at":null,"review_interval_seconds":null,"next_probe_at":null,"escalation_interval_seconds":null,"relative_rules":[]}`, startsAt)
		default:
			fmt.Fprint(responseWriter, `{"unexpected":true}`)
		}
	})
	browserBaseURL := "http://" + browserFixtureAddress
	fixtureRoutes := routes.New(applicationContext, config.EnvConfig{
		CalendarCredentialEncryptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)),
		GoogleClientID:                  "browser-client", GoogleClientSecret: "browser-secret",
		GoogleCalendarAuthorizationEndpoint: browserBaseURL + browserGoogleAuthorizePath,
		GoogleCalendarTokenEndpoint:         browserBaseURL + browserGoogleTokenPath,
		GoogleCalendarListEndpoint:          browserBaseURL + browserGoogleCalendarsPath,
		GoogleCalendarEventsEndpoint:        browserBaseURL + browserGoogleEventsPath,
		NaturalLanguageParserEndpoint:       browserBaseURL + browserNaturalLanguagePath,
		NaturalLanguageParserAPIKey:         "browser-parser-key",
	})
	fixtureRoutes.RegisterRoutes(mux)
	go func() {
		if taskError := fixtureRoutes.RunCalendarConnectionTasks(context.Background()); taskError != nil {
			logger.Printf("Calendar connection task worker stopped: %v", taskError)
		}
	}()
	go func() {
		if synchronizationError := fixtureRoutes.RunCalendarSyncClock(context.Background(), 2*time.Second); synchronizationError != nil {
			logger.Printf("Calendar synchronization clock stopped: %v", synchronizationError)
		}
	}()
	logger.Printf("Listening on http://%s", browserFixtureAddress)
	if serveError := http.ListenAndServe(browserFixtureAddress, mux); serveError != nil {
		logger.Fatalf("Serve browser fixture: %v", serveError)
	}
}

func setBrowserSession(responseWriter http.ResponseWriter, request *http.Request, email string, name string) {
	webSession, sessionError := session.Store().Get(request, gaussConstants.SessionName)
	if sessionError != nil {
		http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	webSession.Values[gaussConstants.SessionKeyUserEmail] = email
	webSession.Values[gaussConstants.SessionKeyUserName] = name
	webSession.Values[gaussConstants.SessionKeyUserPicture] = ""
	if saveError := webSession.Save(request, responseWriter); saveError != nil {
		http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.Redirect(responseWriter, request, config.WebRoot, http.StatusFound)
}

func seedBrowserFixture(database *gorm.DB) error {
	return database.Transaction(func(transaction *gorm.DB) error {
		organizer := models.User{BaseModel: models.BaseModel{ID: browserOrganizerID}, Email: browserOrganizerEmail, Name: "Horizon Browser"}
		if createError := organizer.Create(transaction); createError != nil {
			return fmt.Errorf("create organizer: %w", createError)
		}
		timezone, timezoneError := models.NewTimezone(browserTimezoneName)
		if timezoneError != nil {
			return timezoneError
		}
		if confirmationError := organizer.ConfirmTimezone(transaction, timezone); confirmationError != nil {
			return confirmationError
		}
		venue := models.Venue{BaseModel: models.BaseModel{ID: "VENBRWSR"}, UserID: organizer.ID, Name: "Browser Terminal", Address: "100 Horizon Way"}
		if createError := venue.Create(transaction); createError != nil {
			return fmt.Errorf("create browser venue: %w", createError)
		}

		birthdays, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALBIRTH", "Birthdays", "birthdays", 0)
		if calendarError != nil {
			return calendarError
		}
		holidays, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALHOLID", "Holidays", "holidays", 1)
		if calendarError != nil {
			return calendarError
		}
		travel, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALTRAVL", "Travel", "travel", 2)
		if calendarError != nil {
			return calendarError
		}
		waiting, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALWAIT0", "Waiting", "waiting", 3)
		if calendarError != nil {
			return calendarError
		}
		work, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALWORK0", "Work", "work", 4)
		if calendarError != nil {
			return calendarError
		}
		for index, calendarID := range []string{"CALAAAOX", "CALAAB0X"} {
			if _, calendarError := createBrowserCalendar(transaction, organizer.ID, calendarID, "Color collision "+calendarID, "google-default", 5+index); calendarError != nil {
				return calendarError
			}
		}

		for index, title := range []string{"Ada's birthday", "Lin's birthday", "Maya's birthday"} {
			markerTime := browserReferenceTime.AddDate(0, 0, 12+index*8)
			laneID := fmt.Sprintf("LANBRT%02d", index)
			eventID := fmt.Sprintf("EVTBRT%02d", index)
			lane, laneError := createBrowserFiniteLane(transaction, birthdays.ID, laneID, title, browserReferenceTime.AddDate(0, 0, -3), markerTime, index)
			if laneError != nil {
				return laneError
			}
			if _, eventError := createBrowserPointEvent(transaction, lane.ID, eventID, title, markerTime, models.IndependentEventRelation()); eventError != nil {
				return eventError
			}
		}

		for index, title := range []string{"Founders Day", "Summer Holiday"} {
			markerTime := browserReferenceTime.AddDate(0, 0, 18+index*10)
			laneID := fmt.Sprintf("LANHOL%02d", index)
			eventID := fmt.Sprintf("EVTHOL%02d", index)
			lane, laneError := createBrowserFiniteLane(transaction, holidays.ID, laneID, title, browserReferenceTime.AddDate(0, 0, -2), markerTime, index)
			if laneError != nil {
				return laneError
			}
			if _, eventError := createBrowserPointEvent(transaction, lane.ID, eventID, title, markerTime, models.IndependentEventRelation()); eventError != nil {
				return eventError
			}
		}

		travelLane, laneError := createBrowserFiniteLane(
			transaction, travel.ID, "LANTRAVL", "Coastal trip", browserReferenceTime.AddDate(0, 0, -1), browserReferenceTime.AddDate(0, 0, 28), 0,
		)
		if laneError != nil {
			return laneError
		}
		anchor, anchorError := createBrowserPointEvent(
			transaction, travelLane.ID, "EVTFLGHT", "Flight departs", browserReferenceTime.AddDate(0, 0, 20), models.IndependentEventRelation(),
		)
		if anchorError != nil {
			return anchorError
		}
		if updateError := transaction.Model(anchor).Update("venue_id", venue.ID).Error; updateError != nil {
			return fmt.Errorf("assign browser venue: %w", updateError)
		}
		browserRSVP := models.RSVP{BaseModel: models.BaseModel{ID: "RSVPBR01"}, Name: "Browser Invitee", Response: config.RSVPResponsePending, EventID: anchor.ID}
		if createError := browserRSVP.Create(transaction); createError != nil {
			return fmt.Errorf("create browser RSVP: %w", createError)
		}
		dependentRelation, relationError := models.DependentEventRelation(anchor.ID)
		if relationError != nil {
			return relationError
		}
		if _, dependentError := createBrowserPointEvent(
			transaction, travelLane.ID, "EVTHOTEL", "Hotel check-in", browserReferenceTime.AddDate(0, 0, 21), dependentRelation,
		); dependentError != nil {
			return dependentError
		}

		waitingLane, waitingLaneError := models.NewOpenLane(waiting.ID, "Passport renewal", browserReferenceTime.AddDate(0, 0, -14), 0)
		if waitingLaneError != nil {
			return waitingLaneError
		}
		waitingLane.BaseModel.ID = "LANWAIT0"
		if createError := transaction.Create(waitingLane).Error; createError != nil {
			return fmt.Errorf("create open waiting lane: %w", createError)
		}
		attentionLane, attentionLaneError := models.NewOpenLane(waiting.ID, "Appeal review", browserReferenceTime.AddDate(0, 0, -10), 1)
		if attentionLaneError != nil {
			return attentionLaneError
		}
		attentionLane.BaseModel.ID = "LANATTN0"
		if createError := transaction.Create(attentionLane).Error; createError != nil {
			return fmt.Errorf("create attention lane: %w", createError)
		}
		reviewInterval := 7 * 24 * time.Hour
		escalationInterval := 24 * time.Hour
		policy, policyError := models.NewAttentionPolicy(attentionLane.ID, reviewInterval, browserReferenceTime.AddDate(0, 0, 3), &escalationInterval)
		if policyError != nil {
			return policyError
		}
		policy.BaseModel.ID = "POLWAIT0"
		if createError := transaction.Create(policy).Error; createError != nil {
			return fmt.Errorf("create waiting attention policy: %w", createError)
		}
		escalatesAt := policy.NextProbeAt.Add(escalationInterval)
		pendingProbe, probeError := models.NewProbe(policy.ID, attentionLane.ID, policy.NextProbeAt, &escalatesAt)
		if probeError != nil {
			return probeError
		}
		pendingProbe.BaseModel.ID = "PRBWAIT0"
		if createError := transaction.Create(pendingProbe).Error; createError != nil {
			return fmt.Errorf("create waiting probe: %w", createError)
		}

		seriesLane, seriesLaneError := createBrowserFiniteLane(
			transaction, work.ID, "LANSER00", "Design review series", browserReferenceTime.AddDate(0, 0, -2), browserReferenceTime.AddDate(0, 0, 52), 0,
		)
		if seriesLaneError != nil {
			return seriesLaneError
		}
		recurrenceRule := "FREQ=WEEKLY"
		eventSeries, eventSeriesError := models.NewEventSeries(seriesLane.ID, timezone, models.EventSourceLocal, &recurrenceRule)
		if eventSeriesError != nil {
			return eventSeriesError
		}
		eventSeries.BaseModel.ID = "SERWORK0"
		if createError := transaction.Create(eventSeries).Error; createError != nil {
			return fmt.Errorf("create browser event series: %w", createError)
		}
		seriesRelation, relationError := models.SeriesOccurrenceRelation(eventSeries.ID)
		if relationError != nil {
			return relationError
		}
		for index, eventID := range []string{"EVTSER00", "EVTSER01"} {
			if _, eventError := createBrowserPointEvent(
				transaction, seriesLane.ID, eventID, "Design review", browserReferenceTime.AddDate(0, 0, 14+index*7), seriesRelation,
			); eventError != nil {
				return eventError
			}
		}

		windowStart, windowStartError := browserWindowStart()
		if windowStartError != nil {
			return windowStartError
		}
		if _, laneError := createBrowserFiniteLane(
			transaction, work.ID, "LANLONG0", "Long-running project", windowStart.AddDate(0, 0, -2), windowStart.AddDate(0, 0, 120), 1,
		); laneError != nil {
			return laneError
		}
		boundaryLane, laneError := createBrowserFiniteLane(
			transaction, work.ID, "LANBOUND", "Quarterly estimated tax payment deadline", windowStart, windowStart.AddDate(0, 0, 1), 2,
		)
		if laneError != nil {
			return laneError
		}
		if _, eventError := createBrowserPointEvent(
			transaction, boundaryLane.ID, "EVTBOUND", "Window opens", windowStart, models.IndependentEventRelation(),
		); eventError != nil {
			return eventError
		}
		return nil
	})
}

func browserWindowStart() (time.Time, error) {
	location, locationError := time.LoadLocation(browserTimezoneName)
	if locationError != nil {
		return time.Time{}, locationError
	}
	localReference := browserReferenceTime.In(location)
	return time.Date(localReference.Year(), localReference.Month(), localReference.Day(), 0, 0, 0, 0, location), nil
}

func createBrowserCalendar(database *gorm.DB, organizerID string, identifier string, name string, colorToken string, order int) (*models.Calendar, error) {
	calendar, calendarError := models.NewCalendar(organizerID, name, colorToken, order)
	if calendarError != nil {
		return nil, calendarError
	}
	calendar.BaseModel.ID = identifier
	if createError := database.Create(calendar).Error; createError != nil {
		return nil, fmt.Errorf("create calendar %s: %w", identifier, createError)
	}
	return calendar, nil
}

func createBrowserFiniteLane(database *gorm.DB, calendarID string, identifier string, title string, start time.Time, end time.Time, order int) (*models.Lane, error) {
	lane, laneError := models.NewFiniteLane(calendarID, title, start, end, order)
	if laneError != nil {
		return nil, laneError
	}
	lane.BaseModel.ID = identifier
	if createError := database.Create(lane).Error; createError != nil {
		return nil, fmt.Errorf("create lane %s: %w", identifier, createError)
	}
	return lane, nil
}

func createBrowserPointEvent(database *gorm.DB, laneID string, identifier string, title string, at time.Time, relation models.EventRelation) (*models.Event, error) {
	timezone, timezoneError := models.NewTimezone(browserTimezoneName)
	if timezoneError != nil {
		return nil, timezoneError
	}
	eventTime, timeError := models.NewPointEventTime(at, timezone)
	if timeError != nil {
		return nil, timeError
	}
	event, eventError := models.NewEvent(laneID, title, "", nil, relation, eventTime)
	if eventError != nil {
		return nil, eventError
	}
	event.BaseModel.ID = identifier
	if createError := event.Create(database); createError != nil {
		return nil, fmt.Errorf("create event %s: %w", identifier, createError)
	}
	return event, nil
}
