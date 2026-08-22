package horizon_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gaussConstants "github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/horizon"
	"github.com/tyemirov/RSVP/pkg/routes"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

func TestHorizonDefaultWindowUsesOrganizerCalendarDays(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	referenceTime := time.Date(2030, time.March, 8, 18, 30, 0, 0, time.UTC)

	responseRecorder := requestHorizon(testingContext, fixture, owner, config.WebHorizon, horizonJSONMediaType, referenceTime)
	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("horizon status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	projection := decodeProjection(testingContext, responseRecorder)
	location, locationError := time.LoadLocation(testsupport.TimezoneName)
	if locationError != nil {
		testingContext.Fatalf("load fixture timezone: %v", locationError)
	}
	localReference := referenceTime.In(location)
	wantStart := time.Date(localReference.Year(), localReference.Month(), localReference.Day(), 0, 0, 0, 0, location)
	wantEnd := wantStart.AddDate(0, 0, services.DefaultHorizonWindowDays)
	if projection.Window.Start != wantStart.UTC().Format(time.RFC3339Nano) {
		testingContext.Fatalf("window start = %q, want %q", projection.Window.Start, wantStart.UTC().Format(time.RFC3339Nano))
	}
	if projection.Window.End != wantEnd.UTC().Format(time.RFC3339Nano) {
		testingContext.Fatalf("window end = %q, want %q", projection.Window.End, wantEnd.UTC().Format(time.RFC3339Nano))
	}
	if projection.Window.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("window timezone = %q, want %q", projection.Window.Timezone, testsupport.TimezoneName)
	}
}

func TestHorizonProjectionReturnsOrderedOwnerResourcesAndTypedMarkers(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	confirmTimezone(testingContext, fixture.Database, &otherOwner)

	personal := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", "Personal", 0, true)
	work := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00002", "Work", 1, false)
	emptyCalendar := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00003", "Empty", 2, true)
	otherCalendar := createCalendar(testingContext, fixture.Database, otherOwner.ID, "CAL90001", "Other", 0, true)

	windowStart := time.Date(2030, time.January, 1, 8, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2030, time.January, 10, 8, 0, 0, 0, time.UTC)
	firstLane := createFiniteLane(testingContext, fixture.Database, personal.ID, "LAN00001", "First event", windowStart.Add(-time.Hour), windowStart.Add(2*time.Hour), 0)
	secondLane := createFiniteLane(testingContext, fixture.Database, personal.ID, "LAN00002", "Second event", windowStart.Add(-2*time.Hour), windowStart.Add(3*time.Hour), 1)
	emptyOpenLane := createOpenLane(testingContext, fixture.Database, personal.ID, "LAN00003", "Waiting", windowStart.Add(-24*time.Hour), 2)
	allDayLane := createFiniteLane(testingContext, fixture.Database, personal.ID, "LAN00007", "All day", windowStart.Add(-time.Hour), windowStart.Add(24*time.Hour), 3)
	probeLane := createOpenLane(testingContext, fixture.Database, work.ID, "LAN00004", "Review", windowStart.Add(-48*time.Hour), 0)
	boundaryLane := createFiniteLane(testingContext, fixture.Database, work.ID, "LAN00005", "Boundary", windowStart.Add(-2*time.Hour), windowStart, 1)
	outsideLane := createFiniteLane(testingContext, fixture.Database, work.ID, "LAN00006", "Outside", windowStart.Add(-4*time.Hour), windowStart.Add(-time.Hour), 2)
	otherLane := createFiniteLane(testingContext, fixture.Database, otherCalendar.ID, "LAN90001", "Private", windowStart.Add(-time.Hour), windowStart.Add(time.Hour), 0)

	createPointEvent(testingContext, fixture.Database, firstLane.ID, "EVT00001", "First", windowStart)
	createIntervalEvent(testingContext, fixture.Database, secondLane.ID, "EVT00002", "Second", windowStart.Add(time.Hour), windowStart.Add(2*time.Hour))
	createIntervalEvent(testingContext, fixture.Database, boundaryLane.ID, "EVT00003", "Boundary", windowStart.Add(-time.Hour), windowStart)
	createPointEvent(testingContext, fixture.Database, outsideLane.ID, "EVT00004", "Outside", windowStart.Add(-2*time.Hour))
	createPointEvent(testingContext, fixture.Database, otherLane.ID, "EVT90001", "Private", windowStart)
	createAllDayEvent(testingContext, fixture.Database, allDayLane.ID, "EVT00007", "All day", "2030-01-01", "2030-01-02")
	createProbe(testingContext, fixture.Database, probeLane.ID, "POL00001", "PRB00001", windowStart.Add(4*time.Hour))

	projectionQueryCount := 0
	eventRowsRead := int64(-1)
	projectionQueriesUseOneTransaction := true
	var projectionTransaction *sql.Tx
	queryCallbackName := "test:horizon_projection_snapshot"
	callbackError := fixture.Database.Callback().Query().After("gorm:query").Register(queryCallbackName, func(queryDatabase *gorm.DB) {
		switch queryDatabase.Statement.Table {
		case config.TableCalendars, config.TableLanes, config.TableEvents, config.TableProbes:
		default:
			return
		}
		currentTransaction, transactionFound := queryDatabase.Statement.ConnPool.(*sql.Tx)
		if !transactionFound {
			projectionQueriesUseOneTransaction = false
		} else if projectionTransaction == nil {
			projectionTransaction = currentTransaction
		} else if projectionTransaction != currentTransaction {
			projectionQueriesUseOneTransaction = false
		}
		projectionQueryCount++
		if queryDatabase.Statement.Table == config.TableEvents {
			eventRowsRead = queryDatabase.RowsAffected
		}
	})
	if callbackError != nil {
		testingContext.Fatalf("register projection query callback: %v", callbackError)
	}
	testingContext.Cleanup(func() {
		if removeError := fixture.Database.Callback().Query().Remove(queryCallbackName); removeError != nil {
			testingContext.Errorf("remove projection query callback: %v", removeError)
		}
	})

	target := fmt.Sprintf("%s?%s=%s&%s=%s", config.WebHorizon, config.HorizonStartParam, windowStart.Format(time.RFC3339), config.HorizonEndParam, windowEnd.Format(time.RFC3339))
	responseRecorder := requestHorizon(testingContext, fixture, owner, target, horizonJSONMediaType, time.Now())
	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("horizon status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	assertProjectionHeaders(testingContext, responseRecorder)
	projection := decodeProjection(testingContext, responseRecorder)
	if projectionQueryCount != 4 || !projectionQueriesUseOneTransaction || projectionTransaction == nil {
		testingContext.Fatalf("projection query snapshot count = %d, one transaction = %t", projectionQueryCount, projectionQueriesUseOneTransaction)
	}
	if eventRowsRead != 3 {
		testingContext.Fatalf("event rows read = %d, want 3 in-window candidates", eventRowsRead)
	}
	if len(projection.Calendars) != 3 {
		testingContext.Fatalf("calendar count = %d, want 3", len(projection.Calendars))
	}
	if projection.Calendars[0].ID != personal.ID || projection.Calendars[1].ID != work.ID || projection.Calendars[2].ID != emptyCalendar.ID {
		testingContext.Fatalf("calendar order = [%s, %s, %s], want [%s, %s, %s]", projection.Calendars[0].ID, projection.Calendars[1].ID, projection.Calendars[2].ID, personal.ID, work.ID, emptyCalendar.ID)
	}
	if projection.Calendars[1].Visible {
		testingContext.Fatal("hidden calendar became visible in projection")
	}
	if len(projection.Calendars[0].Lanes) != 4 {
		testingContext.Fatalf("personal lane count = %d, want 4", len(projection.Calendars[0].Lanes))
	}
	if projection.Calendars[0].Lanes[0].ID != firstLane.ID || projection.Calendars[0].Lanes[1].ID != secondLane.ID {
		testingContext.Fatalf("independent lane order = [%s, %s], want [%s, %s]", projection.Calendars[0].Lanes[0].ID, projection.Calendars[0].Lanes[1].ID, firstLane.ID, secondLane.ID)
	}
	if len(projection.Calendars[0].Lanes[2].Markers) != 0 || projection.Calendars[0].Lanes[2].ID != emptyOpenLane.ID {
		testingContext.Fatalf("empty open lane projection = %#v", projection.Calendars[0].Lanes[2])
	}
	if len(projection.Calendars[0].Lanes[0].Markers) != 1 || projection.Calendars[0].Lanes[0].Markers[0].Type != services.HorizonMarkerEvent {
		testingContext.Fatalf("first event markers = %#v", projection.Calendars[0].Lanes[0].Markers)
	}
	if projection.Calendars[0].Lanes[0].Markers[0].Time.Shape != models.EventTimePoint {
		testingContext.Fatalf("first marker shape = %q, want %q", projection.Calendars[0].Lanes[0].Markers[0].Time.Shape, models.EventTimePoint)
	}
	if len(projection.Calendars[0].Lanes[3].Markers) != 1 || projection.Calendars[0].Lanes[3].Markers[0].Time.Shape != models.EventTimeAllDay {
		testingContext.Fatalf("all-day markers = %#v", projection.Calendars[0].Lanes[3].Markers)
	}
	if len(projection.Calendars[1].Lanes) != 2 {
		testingContext.Fatalf("work lane count = %d, want 2", len(projection.Calendars[1].Lanes))
	}
	if len(projection.Calendars[1].Lanes[0].Markers) != 1 || projection.Calendars[1].Lanes[0].Markers[0].Type != services.HorizonMarkerProbe {
		testingContext.Fatalf("probe markers = %#v", projection.Calendars[1].Lanes[0].Markers)
	}
	if len(projection.Calendars[1].Lanes[1].Markers) != 0 {
		testingContext.Fatalf("boundary markers = %#v, want none", projection.Calendars[1].Lanes[1].Markers)
	}
	if len(projection.Calendars[2].Lanes) != 0 {
		testingContext.Fatalf("empty calendar lanes = %#v, want none", projection.Calendars[2].Lanes)
	}
	projectionJSON := responseRecorder.Body.String()
	for _, forbiddenIdentifier := range []string{outsideLane.ID, otherCalendar.ID, otherLane.ID, "EVT90001"} {
		if strings.Contains(projectionJSON, forbiddenIdentifier) {
			testingContext.Fatalf("projection contains excluded identifier %q", forbiddenIdentifier)
		}
	}
}

func TestHorizonAcceptsMaximumWindow(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	location, locationError := time.LoadLocation(testsupport.TimezoneName)
	if locationError != nil {
		testingContext.Fatalf("load fixture timezone: %v", locationError)
	}
	start := time.Date(2030, time.March, 8, 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, services.MaximumHorizonWindowDays)
	target := config.WebHorizon + "?start=" + start.Format(time.RFC3339) + "&end=" + end.Format(time.RFC3339)
	responseRecorder := requestHorizon(testingContext, fixture, owner, target, horizonJSONMediaType, time.Now())
	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("maximum window status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	projection := decodeProjection(testingContext, responseRecorder)
	if projection.Window.End != end.UTC().Format(time.RFC3339Nano) {
		testingContext.Fatalf("maximum window end = %q, want %q", projection.Window.End, end.UTC().Format(time.RFC3339Nano))
	}
}

func TestHorizonHTMLAndJSONContainOneProjection(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	testsupport.LoadTemplates(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendar := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", "Personal", 0, true)
	windowStart := time.Date(2030, time.January, 1, 8, 0, 0, 0, time.UTC)
	lane := createFiniteLane(testingContext, fixture.Database, calendar.ID, "LAN00001", "Birthday", windowStart.Add(-time.Hour), windowStart.Add(2*time.Hour), 0)
	createPointEvent(testingContext, fixture.Database, lane.ID, "EVT00001", "Birthday", windowStart)
	target := config.WebHorizon + "?start=" + windowStart.Format(time.RFC3339) + "&end=" + windowStart.Add(24*time.Hour).Format(time.RFC3339)

	jsonResponse := requestHorizon(testingContext, fixture, owner, target, horizonJSONMediaType, time.Now())
	htmlResponse := requestHorizon(testingContext, fixture, owner, target, horizonHTMLMediaType, time.Now())
	for name, responseRecorder := range map[string]*httptest.ResponseRecorder{"JSON": jsonResponse, "HTML": htmlResponse} {
		if responseRecorder.Code != http.StatusOK {
			testingContext.Fatalf("%s status = %d, want %d; body = %s", name, responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
		}
		assertProjectionHeaders(testingContext, responseRecorder)
		for _, identifier := range []string{calendar.ID, lane.ID, "EVT00001"} {
			if !strings.Contains(responseRecorder.Body.String(), identifier) {
				testingContext.Fatalf("%s response does not contain %q", name, identifier)
			}
		}
	}
	if jsonResponse.Header().Get("Content-Type") != horizonJSONMediaType {
		testingContext.Fatalf("JSON content type = %q, want %q", jsonResponse.Header().Get("Content-Type"), horizonJSONMediaType)
	}
	if !strings.HasPrefix(htmlResponse.Header().Get("Content-Type"), horizonHTMLMediaType) {
		testingContext.Fatalf("HTML content type = %q, want %q", htmlResponse.Header().Get("Content-Type"), horizonHTMLMediaType)
	}
}

func TestHorizonRejectsInvalidWindowsWithTypedError(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	start := time.Date(2030, time.January, 1, 8, 0, 0, 0, time.UTC)
	location, locationError := time.LoadLocation(testsupport.TimezoneName)
	if locationError != nil {
		testingContext.Fatalf("load fixture timezone: %v", locationError)
	}
	tooLargeEnd := start.In(location).AddDate(0, 0, services.MaximumHorizonWindowDays+1)
	testCases := map[string]struct {
		target     string
		wantStatus int
	}{
		"missing end":   {target: config.WebHorizon + "?start=" + start.Format(time.RFC3339), wantStatus: http.StatusBadRequest},
		"invalid start": {target: config.WebHorizon + "?start=not-a-time&end=" + start.Add(time.Hour).Format(time.RFC3339), wantStatus: http.StatusBadRequest},
		"reversed":      {target: config.WebHorizon + "?start=" + start.Format(time.RFC3339) + "&end=" + start.Add(-time.Hour).Format(time.RFC3339), wantStatus: http.StatusUnprocessableEntity},
		"too large":     {target: config.WebHorizon + "?start=" + start.Format(time.RFC3339) + "&end=" + tooLargeEnd.Format(time.RFC3339), wantStatus: http.StatusUnprocessableEntity},
	}
	for testName, testCase := range testCases {
		testingContext.Run(testName, func(testingContext *testing.T) {
			responseRecorder := requestHorizon(testingContext, fixture, owner, testCase.target, horizonJSONMediaType, time.Now())
			if responseRecorder.Code != testCase.wantStatus {
				testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, testCase.wantStatus, responseRecorder.Body.String())
			}
			var responseBody struct {
				Error struct {
					Code      string            `json:"code"`
					Message   string            `json:"message"`
					Details   map[string]string `json:"details"`
					RequestID string            `json:"request_id"`
				} `json:"error"`
			}
			if decodeError := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); decodeError != nil {
				testingContext.Fatalf("decode typed error: %v", decodeError)
			}
			if responseBody.Error.Code != "invalid_time_window" || responseBody.Error.Message != "The time window is invalid." {
				testingContext.Fatalf("typed error = %#v", responseBody.Error)
			}
			if responseBody.Error.Details == nil || len(responseBody.Error.Details) != 0 || responseBody.Error.RequestID == "" {
				testingContext.Fatalf("typed error details = %#v, request ID = %q", responseBody.Error.Details, responseBody.Error.RequestID)
			}
		})
	}
}

func TestHorizonRouteRequiresAuthentication(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	mux := http.NewServeMux()
	routes.New(fixture.ApplicationContext, config.EnvConfig{}).RegisterRoutes(mux)
	testServer := httptest.NewServer(mux)
	testingContext.Cleanup(testServer.Close)
	testClient := testServer.Client()
	testClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	unauthenticatedResponse, requestError := testClient.Get(testServer.URL + config.WebHorizon)
	if requestError != nil {
		testingContext.Fatalf("request unauthenticated horizon: %v", requestError)
	}
	if closeError := unauthenticatedResponse.Body.Close(); closeError != nil {
		testingContext.Fatalf("close unauthenticated response: %v", closeError)
	}
	if unauthenticatedResponse.StatusCode != http.StatusFound {
		testingContext.Fatalf("unauthenticated status = %d, want %d", unauthenticatedResponse.StatusCode, http.StatusFound)
	}
	unauthenticatedJSONRequest, requestConstructionError := http.NewRequest(http.MethodGet, testServer.URL+config.WebHorizon, nil)
	if requestConstructionError != nil {
		testingContext.Fatalf("construct unauthenticated JSON request: %v", requestConstructionError)
	}
	unauthenticatedJSONRequest.Header.Set("Accept", horizonJSONMediaType)
	unauthenticatedJSONResponse, jsonRequestError := testClient.Do(unauthenticatedJSONRequest)
	if jsonRequestError != nil {
		testingContext.Fatalf("request unauthenticated JSON horizon: %v", jsonRequestError)
	}
	var authenticationErrorResponse struct {
		Error struct {
			Code      string            `json:"code"`
			Details   map[string]string `json:"details"`
			RequestID string            `json:"request_id"`
		} `json:"error"`
	}
	if decodeError := json.NewDecoder(unauthenticatedJSONResponse.Body).Decode(&authenticationErrorResponse); decodeError != nil {
		testingContext.Fatalf("decode unauthenticated JSON response: %v", decodeError)
	}
	if closeError := unauthenticatedJSONResponse.Body.Close(); closeError != nil {
		testingContext.Fatalf("close unauthenticated JSON response: %v", closeError)
	}
	if unauthenticatedJSONResponse.StatusCode != http.StatusUnauthorized {
		testingContext.Fatalf("unauthenticated JSON status = %d, want %d", unauthenticatedJSONResponse.StatusCode, http.StatusUnauthorized)
	}
	if authenticationErrorResponse.Error.Code != "authentication_required" || authenticationErrorResponse.Error.Details == nil || authenticationErrorResponse.Error.RequestID == "" {
		testingContext.Fatalf("authentication error response = %#v", authenticationErrorResponse.Error)
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, config.WebHorizon, nil)
	sessionRecorder := httptest.NewRecorder()
	webSession, sessionError := session.Store().Get(sessionRequest, gaussConstants.SessionName)
	if sessionError != nil {
		testingContext.Fatalf("get authenticated session: %v", sessionError)
	}
	webSession.Values[gaussConstants.SessionKeyUserEmail] = owner.Email
	webSession.Values[gaussConstants.SessionKeyUserName] = owner.Name
	if saveError := webSession.Save(sessionRequest, sessionRecorder); saveError != nil {
		testingContext.Fatalf("save authenticated session: %v", saveError)
	}
	authenticatedRequest, requestConstructionError := http.NewRequest(http.MethodGet, testServer.URL+config.WebHorizon, nil)
	if requestConstructionError != nil {
		testingContext.Fatalf("construct authenticated request: %v", requestConstructionError)
	}
	authenticatedRequest.Header.Set("Accept", horizonJSONMediaType)
	for _, cookie := range sessionRecorder.Result().Cookies() {
		authenticatedRequest.AddCookie(cookie)
	}
	authenticatedResponse, authenticatedRequestError := testClient.Do(authenticatedRequest)
	if authenticatedRequestError != nil {
		testingContext.Fatalf("request authenticated horizon: %v", authenticatedRequestError)
	}
	authenticatedBody, bodyError := io.ReadAll(authenticatedResponse.Body)
	if bodyError != nil {
		testingContext.Fatalf("read authenticated response: %v", bodyError)
	}
	if closeError := authenticatedResponse.Body.Close(); closeError != nil {
		testingContext.Fatalf("close authenticated response: %v", closeError)
	}
	if authenticatedResponse.StatusCode != http.StatusOK {
		testingContext.Fatalf("authenticated status = %d, want %d; body = %s", authenticatedResponse.StatusCode, http.StatusOK, authenticatedBody)
	}
}

func TestHorizonRejectsUnsupportedMethodRepresentationAndPath(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	testCases := []struct {
		name       string
		method     string
		target     string
		accept     string
		wantStatus int
		wantAllow  string
	}{
		{name: "method", method: http.MethodPost, target: config.WebHorizon, accept: horizonJSONMediaType, wantStatus: http.StatusMethodNotAllowed, wantAllow: http.MethodGet},
		{name: "representation", method: http.MethodGet, target: config.WebHorizon, accept: "application/xml", wantStatus: http.StatusNotAcceptable},
		{name: "path", method: http.MethodGet, target: config.WebHorizon + "other", accept: horizonJSONMediaType, wantStatus: http.StatusNotFound},
	}
	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			request := testsupport.Request(testingContext, testCase.method, testCase.target, nil, &owner)
			request.Header.Set("Accept", testCase.accept)
			responseRecorder := httptest.NewRecorder()
			horizon.Handler(fixture.ApplicationContext, time.Now).ServeHTTP(responseRecorder, request)
			if responseRecorder.Code != testCase.wantStatus {
				testingContext.Fatalf("status = %d, want %d", responseRecorder.Code, testCase.wantStatus)
			}
			if responseRecorder.Header().Get("Allow") != testCase.wantAllow {
				testingContext.Fatalf("Allow = %q, want %q", responseRecorder.Header().Get("Allow"), testCase.wantAllow)
			}
		})
	}
}

const (
	horizonJSONMediaType = "application/json"
	horizonHTMLMediaType = "text/html"
)

func requestHorizon(
	testingContext *testing.T,
	fixture *testsupport.Fixture,
	owner models.User,
	target string,
	accept string,
	referenceTime time.Time,
) *httptest.ResponseRecorder {
	testingContext.Helper()
	request := testsupport.Request(testingContext, http.MethodGet, target, nil, &owner)
	request.Header.Set("Accept", accept)
	responseRecorder := httptest.NewRecorder()
	horizon.Handler(fixture.ApplicationContext, func() time.Time { return referenceTime }).ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func decodeProjection(testingContext *testing.T, responseRecorder *httptest.ResponseRecorder) services.HorizonProjection {
	testingContext.Helper()
	var projection services.HorizonProjection
	if decodeError := json.Unmarshal(responseRecorder.Body.Bytes(), &projection); decodeError != nil {
		testingContext.Fatalf("decode horizon projection: %v", decodeError)
	}
	return projection
}

func assertProjectionHeaders(testingContext *testing.T, responseRecorder *httptest.ResponseRecorder) {
	testingContext.Helper()
	if responseRecorder.Header().Get("Cache-Control") != "private, no-store" {
		testingContext.Fatalf("Cache-Control = %q, want %q", responseRecorder.Header().Get("Cache-Control"), "private, no-store")
	}
	if responseRecorder.Header().Get("Vary") != "Accept" {
		testingContext.Fatalf("Vary = %q, want %q", responseRecorder.Header().Get("Vary"), "Accept")
	}
}

func confirmTimezone(testingContext *testing.T, database *gorm.DB, owner *models.User) {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct fixture timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm fixture timezone: %v", confirmationError)
	}
}

func createCalendar(testingContext *testing.T, database *gorm.DB, ownerID string, identifier string, name string, displayOrder int, visible bool) models.Calendar {
	testingContext.Helper()
	calendar, calendarError := models.NewCalendar(ownerID, name, strings.ToLower(name), "test", displayOrder)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar %s: %v", identifier, calendarError)
	}
	calendar.BaseModel.ID = identifier
	if createError := database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar %s: %v", identifier, createError)
	}
	if !visible {
		if updateError := database.Model(calendar).Update("visible", false).Error; updateError != nil {
			testingContext.Fatalf("hide calendar %s: %v", identifier, updateError)
		}
		calendar.Visible = false
	}
	return *calendar
}

func createFiniteLane(testingContext *testing.T, database *gorm.DB, calendarID string, identifier string, title string, start time.Time, end time.Time, displayOrder int) models.Lane {
	testingContext.Helper()
	lane, laneError := models.NewFiniteLane(calendarID, title, start, end, displayOrder)
	if laneError != nil {
		testingContext.Fatalf("construct finite lane %s: %v", identifier, laneError)
	}
	lane.BaseModel.ID = identifier
	if createError := database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create finite lane %s: %v", identifier, createError)
	}
	return *lane
}

func createOpenLane(testingContext *testing.T, database *gorm.DB, calendarID string, identifier string, title string, start time.Time, displayOrder int) models.Lane {
	testingContext.Helper()
	lane, laneError := models.NewOpenLane(calendarID, title, start, displayOrder)
	if laneError != nil {
		testingContext.Fatalf("construct open lane %s: %v", identifier, laneError)
	}
	lane.BaseModel.ID = identifier
	if createError := database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create open lane %s: %v", identifier, createError)
	}
	return *lane
}

func createPointEvent(testingContext *testing.T, database *gorm.DB, laneID string, identifier string, title string, at time.Time) models.Event {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct event timezone: %v", timezoneError)
	}
	eventTime, timeError := models.NewPointEventTime(at, timezone)
	if timeError != nil {
		testingContext.Fatalf("construct point time %s: %v", identifier, timeError)
	}
	event, eventError := models.NewEvent(laneID, title, "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct point event %s: %v", identifier, eventError)
	}
	event.BaseModel.ID = identifier
	if createError := event.Create(database); createError != nil {
		testingContext.Fatalf("create point event %s: %v", identifier, createError)
	}
	return *event
}

func createIntervalEvent(testingContext *testing.T, database *gorm.DB, laneID string, identifier string, title string, start time.Time, end time.Time) models.Event {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct event timezone: %v", timezoneError)
	}
	eventTime, timeError := models.NewIntervalEventTime(start, end, timezone)
	if timeError != nil {
		testingContext.Fatalf("construct interval time %s: %v", identifier, timeError)
	}
	event, eventError := models.NewEvent(laneID, title, "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct interval event %s: %v", identifier, eventError)
	}
	event.BaseModel.ID = identifier
	if createError := event.Create(database); createError != nil {
		testingContext.Fatalf("create interval event %s: %v", identifier, createError)
	}
	return *event
}

func createAllDayEvent(testingContext *testing.T, database *gorm.DB, laneID string, identifier string, title string, startDateValue string, endDateValue string) models.Event {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct event timezone: %v", timezoneError)
	}
	startDate, startDateError := models.NewLocalDate(startDateValue)
	if startDateError != nil {
		testingContext.Fatalf("construct all-day start %s: %v", identifier, startDateError)
	}
	endDate, endDateError := models.NewLocalDate(endDateValue)
	if endDateError != nil {
		testingContext.Fatalf("construct all-day end %s: %v", identifier, endDateError)
	}
	eventTime, timeError := models.NewAllDayEventTime(startDate, endDate, timezone)
	if timeError != nil {
		testingContext.Fatalf("construct all-day time %s: %v", identifier, timeError)
	}
	event, eventError := models.NewEvent(laneID, title, "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct all-day event %s: %v", identifier, eventError)
	}
	event.BaseModel.ID = identifier
	if createError := event.Create(database); createError != nil {
		testingContext.Fatalf("create all-day event %s: %v", identifier, createError)
	}
	return *event
}

func createProbe(testingContext *testing.T, database *gorm.DB, laneID string, policyID string, probeID string, dueAt time.Time) models.Probe {
	testingContext.Helper()
	policy, policyError := models.NewAttentionPolicy(laneID, 7*24*time.Hour, dueAt, nil)
	if policyError != nil {
		testingContext.Fatalf("construct attention policy %s: %v", policyID, policyError)
	}
	policy.BaseModel.ID = policyID
	if createError := database.Create(policy).Error; createError != nil {
		testingContext.Fatalf("create attention policy %s: %v", policyID, createError)
	}
	probe, probeError := models.NewProbe(policy.ID, laneID, dueAt, nil)
	if probeError != nil {
		testingContext.Fatalf("construct probe %s: %v", probeID, probeError)
	}
	probe.BaseModel.ID = probeID
	if createError := database.Create(probe).Error; createError != nil {
		testingContext.Fatalf("create probe %s: %v", probeID, createError)
	}
	return *probe
}
