// Command browserfixture serves deterministic horizon data for real-browser tests.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	browserFixtureAddress = "127.0.0.1:18080"
	browserOrganizerID    = "USRBRWSR"
	browserOrganizerEmail = "horizon@example.test"
	browserTimezoneName   = "America/Los_Angeles"
	browserLoginPath      = "/browser-login/"
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
		webSession, sessionError := session.Store().Get(request, gaussConstants.SessionName)
		if sessionError != nil {
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		webSession.Values[gaussConstants.SessionKeyUserEmail] = browserOrganizerEmail
		webSession.Values[gaussConstants.SessionKeyUserName] = "Horizon Browser"
		webSession.Values[gaussConstants.SessionKeyUserPicture] = ""
		if saveError := webSession.Save(request, responseWriter); saveError != nil {
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		http.Redirect(responseWriter, request, config.WebRoot, http.StatusFound)
	})
	routes.New(applicationContext, config.EnvConfig{}).RegisterRoutes(mux)
	logger.Printf("Listening on http://%s", browserFixtureAddress)
	if serveError := http.ListenAndServe(browserFixtureAddress, mux); serveError != nil {
		logger.Fatalf("Serve browser fixture: %v", serveError)
	}
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

		birthdays, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALBIRTH", "Birthdays", "✦", "birthdays", 0)
		if calendarError != nil {
			return calendarError
		}
		holidays, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALHOLID", "Holidays", "◆", "holidays", 1)
		if calendarError != nil {
			return calendarError
		}
		travel, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALTRAVL", "Travel", "→", "travel", 2)
		if calendarError != nil {
			return calendarError
		}
		waiting, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALWAIT0", "Waiting", "○", "waiting", 3)
		if calendarError != nil {
			return calendarError
		}
		work, calendarError := createBrowserCalendar(transaction, organizer.ID, "CALWORK0", "Work", "□", "work", 4)
		if calendarError != nil {
			return calendarError
		}

		for index, title := range []string{"Ada's birthday", "Lin's birthday", "Maya's birthday"} {
			markerTime := browserReferenceTime.AddDate(0, 0, 12+index*10)
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
			markerTime := browserReferenceTime.AddDate(0, 0, 18+index*16)
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
				transaction, seriesLane.ID, eventID, "Design review", browserReferenceTime.AddDate(0, 0, 30+index*7), seriesRelation,
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
			transaction, work.ID, "LANBOUND", "Window boundary", windowStart, windowStart.AddDate(0, 0, 1), 2,
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

func createBrowserCalendar(database *gorm.DB, organizerID string, identifier string, name string, symbol string, colorToken string, order int) (*models.Calendar, error) {
	calendar, calendarError := models.NewCalendar(organizerID, name, symbol, colorToken, order)
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
