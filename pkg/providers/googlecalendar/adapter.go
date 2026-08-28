// Package googlecalendar connects RSVP to the Google Calendar HTTP API.
package googlecalendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/services"
)

const googleContactsBirthdayCalendarIDSuffix = "#contacts@group.v.calendar.google.com"

// Config contains the Google Calendar adapter boundary values.
type Config struct {
	ClientID              string
	ClientSecret          string
	AuthorizationEndpoint string
	TokenEndpoint         string
	CalendarListEndpoint  string
	EventsEndpoint        string
}

// Adapter sends Google Calendar consent and calendar-list requests.
type Adapter struct {
	config Config
	client *http.Client
	now    func() time.Time
}

// New constructs one Google Calendar adapter.
func New(adapterConfig Config, client *http.Client, now func() time.Time) (*Adapter, error) {
	if client == nil || now == nil || adapterConfig.ClientID == "" || adapterConfig.ClientSecret == "" {
		return nil, errors.New("Google Calendar client configuration is required")
	}
	for _, endpoint := range []string{adapterConfig.AuthorizationEndpoint, adapterConfig.TokenEndpoint, adapterConfig.CalendarListEndpoint, adapterConfig.EventsEndpoint} {
		parsedURL, parseError := url.Parse(endpoint)
		if parseError != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, errors.New("Google Calendar endpoint is invalid")
		}
	}
	return &Adapter{config: adapterConfig, client: client, now: now}, nil
}

// DefaultConfig returns the current Google Calendar endpoints.
func DefaultConfig(clientID string, clientSecret string) Config {
	return Config{
		ClientID: clientID, ClientSecret: clientSecret,
		AuthorizationEndpoint: config.GoogleCalendarAuthorizationEndpoint,
		TokenEndpoint:         config.GoogleCalendarTokenEndpoint,
		CalendarListEndpoint:  config.GoogleCalendarListEndpoint,
		EventsEndpoint:        config.GoogleCalendarEventsEndpoint,
	}
}

// AuthorizationURL returns one read-only Google Calendar consent URL.
func (adapter *Adapter) AuthorizationURL(state string, redirectURI string) (string, error) {
	if state == "" || redirectURI == "" {
		return "", errors.New("Google Calendar consent state and redirect URI are required")
	}
	parsedURL, parseError := url.Parse(adapter.config.AuthorizationEndpoint)
	if parseError != nil {
		return "", fmt.Errorf("parse Google Calendar authorization endpoint: %w", parseError)
	}
	query := parsedURL.Query()
	query.Set("client_id", adapter.config.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	query.Set("state", state)
	query.Set("scope", strings.Join([]string{config.GoogleCalendarListReadonlyScope, config.GoogleCalendarEventsReadonlyScope}, " "))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

// ExchangeCode exchanges one consent code without logging its value.
func (adapter *Adapter) ExchangeCode(ctx context.Context, code string, redirectURI string) (services.CalendarProviderCredential, error) {
	if code == "" || redirectURI == "" {
		return services.CalendarProviderCredential{}, errors.New("Google Calendar authorization code and redirect URI are required")
	}
	form := url.Values{
		"client_id": {adapter.config.ClientID}, "client_secret": {adapter.config.ClientSecret},
		"code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	request, requestError := http.NewRequestWithContext(ctx, http.MethodPost, adapter.config.TokenEndpoint, strings.NewReader(form.Encode()))
	if requestError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("create Google Calendar token request: %w", requestError)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, responseError := adapter.client.Do(request)
	if responseError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("exchange Google Calendar authorization code: %w", responseError)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return services.CalendarProviderCredential{}, fmt.Errorf("exchange Google Calendar authorization code: provider status %d", response.StatusCode)
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if decodeError := json.NewDecoder(response.Body).Decode(&body); decodeError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("decode Google Calendar token response: %w", decodeError)
	}
	if body.AccessToken == "" || body.RefreshToken == "" || body.ExpiresIn <= 0 {
		return services.CalendarProviderCredential{}, errors.New("Google Calendar token response is invalid")
	}
	return services.CalendarProviderCredential{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken, ExpiresAt: adapter.now().UTC().Add(time.Duration(body.ExpiresIn) * time.Second)}, nil
}

// RefreshCredential renews one expired Google Calendar access grant.
func (adapter *Adapter) RefreshCredential(ctx context.Context, credential services.CalendarProviderCredential) (services.CalendarProviderCredential, error) {
	if credential.RefreshToken == "" {
		return services.CalendarProviderCredential{}, errors.New("Google Calendar refresh token is required")
	}
	form := url.Values{
		"client_id": {adapter.config.ClientID}, "client_secret": {adapter.config.ClientSecret},
		"refresh_token": {credential.RefreshToken}, "grant_type": {"refresh_token"},
	}
	request, requestError := http.NewRequestWithContext(ctx, http.MethodPost, adapter.config.TokenEndpoint, strings.NewReader(form.Encode()))
	if requestError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("create Google Calendar refresh request: %w", requestError)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, responseError := adapter.client.Do(request)
	if responseError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("refresh Google Calendar credential: %w", responseError)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return services.CalendarProviderCredential{}, fmt.Errorf("refresh Google Calendar credential: provider status %d", response.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if decodeError := json.NewDecoder(response.Body).Decode(&body); decodeError != nil {
		return services.CalendarProviderCredential{}, fmt.Errorf("decode Google Calendar refresh response: %w", decodeError)
	}
	if body.AccessToken == "" || body.ExpiresIn <= 0 {
		return services.CalendarProviderCredential{}, errors.New("Google Calendar refresh response is invalid")
	}
	return services.CalendarProviderCredential{AccessToken: body.AccessToken, RefreshToken: credential.RefreshToken, ExpiresAt: adapter.now().UTC().Add(time.Duration(body.ExpiresIn) * time.Second)}, nil
}

// ListCalendars returns all CalendarList pages for one cursor transition.
func (adapter *Adapter) ListCalendars(ctx context.Context, credential services.CalendarProviderCredential, syncCursor string) (services.ProviderCalendarBatch, error) {
	if credential.AccessToken == "" {
		return services.ProviderCalendarBatch{}, errors.New("Google Calendar access token is required")
	}
	calendars := make([]services.ProviderCalendar, 0)
	nextPageToken := ""
	for {
		parsedURL, parseError := url.Parse(adapter.config.CalendarListEndpoint)
		if parseError != nil {
			return services.ProviderCalendarBatch{}, fmt.Errorf("parse Google Calendar list endpoint: %w", parseError)
		}
		query := parsedURL.Query()
		query.Set("showHidden", "true")
		if syncCursor != "" {
			query.Set("syncToken", syncCursor)
		}
		if nextPageToken != "" {
			query.Set("pageToken", nextPageToken)
		}
		parsedURL.RawQuery = query.Encode()
		request, requestError := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
		if requestError != nil {
			return services.ProviderCalendarBatch{}, fmt.Errorf("create Google Calendar list request: %w", requestError)
		}
		request.Header.Set("Authorization", "Bearer "+credential.AccessToken)
		response, responseError := adapter.client.Do(request)
		if responseError != nil {
			return services.ProviderCalendarBatch{}, fmt.Errorf("list Google calendars: %w", responseError)
		}
		if response.StatusCode == http.StatusGone {
			response.Body.Close()
			return services.ProviderCalendarBatch{}, services.ErrCalendarListSyncCursorRejected
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return services.ProviderCalendarBatch{}, fmt.Errorf("list Google calendars: provider status %d", response.StatusCode)
		}
		var body struct {
			Items         []googleCalendarListItem `json:"items"`
			NextPageToken string                   `json:"nextPageToken"`
			NextSyncToken string                   `json:"nextSyncToken"`
		}
		decodeError := json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if decodeError != nil {
			return services.ProviderCalendarBatch{}, fmt.Errorf("decode Google Calendar list response: %w", decodeError)
		}
		for _, item := range body.Items {
			if item.ID == "" {
				return services.ProviderCalendarBatch{}, errors.New("Google Calendar list response is invalid")
			}
			if item.Deleted {
				calendars = append(calendars, services.ProviderCalendar{ID: item.ID, Deleted: true})
				continue
			}
			name := item.SummaryOverride
			if name == "" {
				name = item.Summary
			}
			readable, accessRoleValid := readableCalendarAccessRole(item.AccessRole)
			if name == "" || !accessRoleValid {
				return services.ProviderCalendarBatch{}, errors.New("Google Calendar list response is invalid")
			}
			colorToken := strings.TrimPrefix(item.BackgroundColor, "#")
			if colorToken == "" {
				colorToken = "google-default"
			}
			groups := normalizedGoogleCalendarGroups(item, name, colorToken)
			calendars = append(calendars, services.ProviderCalendar{ID: item.ID, Readable: readable && len(groups) != 0, Groups: groups})
		}
		nextPageToken = body.NextPageToken
		if nextPageToken == "" {
			if body.NextSyncToken == "" {
				return services.ProviderCalendarBatch{}, errors.New("Google Calendar list response has no final sync cursor")
			}
			return services.ProviderCalendarBatch{Calendars: calendars, NextSyncCursor: body.NextSyncToken}, nil
		}
	}
}

type googleCalendarListItem struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	SummaryOverride string `json:"summaryOverride"`
	Timezone        string `json:"timeZone"`
	BackgroundColor string `json:"backgroundColor"`
	Selected        bool   `json:"selected"`
	Deleted         bool   `json:"deleted"`
	AccessRole      string `json:"accessRole"`
	Primary         bool   `json:"primary"`
}

func normalizedGoogleCalendarGroups(item googleCalendarListItem, name string, colorToken string) []services.ProviderCalendarGroup {
	if strings.HasSuffix(item.ID, googleContactsBirthdayCalendarIDSuffix) {
		return nil
	}
	groups := []services.ProviderCalendarGroup{{
		Key: services.ProviderCalendarGroupCalendar, Name: name, ColorToken: colorToken, Visible: item.Selected,
	}}
	if item.Primary {
		groups = append(groups, services.ProviderCalendarGroup{
			Key: services.ProviderCalendarGroupBirthdays, Name: "Birthdays", ColorToken: "birthdays", Visible: true,
		})
	}
	return groups
}

func readableCalendarAccessRole(accessRole string) (bool, bool) {
	switch accessRole {
	case "freeBusyReader":
		return false, true
	case "reader", "writerWithoutPrivateAccess", "writer", "owner":
		return true, true
	default:
		return false, false
	}
}

// SynchronizeEvents returns all pages for one initial or incremental cursor transition.
func (adapter *Adapter) SynchronizeEvents(ctx context.Context, credential services.CalendarProviderCredential, providerCalendarID string, providerGroup services.ProviderCalendarGroupKey, syncCursor string) (services.ProviderEventBatch, error) {
	if credential.AccessToken == "" || providerCalendarID == "" {
		return services.ProviderEventBatch{}, errors.New("Google Calendar event synchronization values are required")
	}
	if groupError := validateGoogleEventGroup(providerGroup); groupError != nil {
		return services.ProviderEventBatch{}, groupError
	}
	events := make([]services.ProviderEvent, 0)
	nextPageToken := ""
	for {
		endpoint := strings.TrimRight(adapter.config.EventsEndpoint, "/") + "/" + url.PathEscape(providerCalendarID) + "/events"
		parsedURL, parseError := url.Parse(endpoint)
		if parseError != nil {
			return services.ProviderEventBatch{}, fmt.Errorf("parse Google Calendar events endpoint: %w", parseError)
		}
		query := parsedURL.Query()
		query.Set("singleEvents", "true")
		query.Set("showDeleted", "true")
		if syncCursor != "" {
			query.Set("syncToken", syncCursor)
		}
		if nextPageToken != "" {
			query.Set("pageToken", nextPageToken)
		}
		parsedURL.RawQuery = query.Encode()
		request, requestError := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
		if requestError != nil {
			return services.ProviderEventBatch{}, fmt.Errorf("create Google Calendar events request: %w", requestError)
		}
		request.Header.Set("Authorization", "Bearer "+credential.AccessToken)
		response, responseError := adapter.client.Do(request)
		if responseError != nil {
			return services.ProviderEventBatch{}, fmt.Errorf("synchronize Google Calendar events: %w", responseError)
		}
		if response.StatusCode == http.StatusGone {
			response.Body.Close()
			return services.ProviderEventBatch{}, services.ErrCalendarSyncCursorRejected
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return services.ProviderEventBatch{}, fmt.Errorf("synchronize Google Calendar events: provider status %d", response.StatusCode)
		}
		var body googleEventsResponse
		decodeError := json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if decodeError != nil {
			return services.ProviderEventBatch{}, fmt.Errorf("decode Google Calendar events response: %w", decodeError)
		}
		for _, item := range body.Items {
			if item.ID == "" {
				return services.ProviderEventBatch{}, errors.New("Google Calendar event response is invalid")
			}
			if !googleEventBelongsToGroup(item, providerGroup) {
				events = append(events, services.ProviderEvent{ID: item.ID, Status: "cancelled"})
				continue
			}
			providerEvent, eventError := decodeProviderEvent(item, body.Timezone)
			if eventError != nil {
				return services.ProviderEventBatch{}, eventError
			}
			events = append(events, providerEvent)
		}
		nextPageToken = body.NextPageToken
		if nextPageToken == "" {
			if body.NextSyncToken == "" {
				return services.ProviderEventBatch{}, errors.New("Google Calendar events response has no final sync cursor")
			}
			return services.ProviderEventBatch{Events: events, NextSyncCursor: body.NextSyncToken}, nil
		}
	}
}

func validateGoogleEventGroup(providerGroup services.ProviderCalendarGroupKey) error {
	switch providerGroup {
	case services.ProviderCalendarGroupCalendar, services.ProviderCalendarGroupBirthdays:
		return nil
	default:
		return errors.New("Google Calendar group is invalid")
	}
}

func googleEventBelongsToGroup(item googleEventItem, providerGroup services.ProviderCalendarGroupKey) bool {
	if item.Status == "cancelled" {
		return true
	}
	isBirthday := item.EventType == "birthday" || item.BirthdayProperties != nil || googleTitleHasBirthdayMeaning(item.Summary)
	if providerGroup == services.ProviderCalendarGroupBirthdays {
		return isBirthday
	}
	return !isBirthday
}

func googleTitleHasBirthdayMeaning(title string) bool {
	words := strings.FieldsFunc(strings.ToLower(title), func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	})
	for _, word := range words {
		switch word {
		case "birthday", "birthdays", "bday":
			return true
		}
	}
	return false
}

type googleEventsResponse struct {
	Timezone      string            `json:"timeZone"`
	Items         []googleEventItem `json:"items"`
	NextPageToken string            `json:"nextPageToken"`
	NextSyncToken string            `json:"nextSyncToken"`
}

type googleEventItem struct {
	ID                 string    `json:"id"`
	RecurringEventID   string    `json:"recurringEventId"`
	EventType          string    `json:"eventType"`
	Status             string    `json:"status"`
	Summary            string    `json:"summary"`
	Description        string    `json:"description"`
	BirthdayProperties *struct{} `json:"birthdayProperties"`
	Start              struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
		Timezone string `json:"timeZone"`
	} `json:"start"`
	End struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
		Timezone string `json:"timeZone"`
	} `json:"end"`
}

func decodeProviderEvent(item googleEventItem, calendarTimezone string) (services.ProviderEvent, error) {
	if item.ID == "" {
		return services.ProviderEvent{}, errors.New("Google Calendar event response is invalid")
	}
	event := services.ProviderEvent{ID: item.ID, SeriesID: item.RecurringEventID, Title: item.Summary, Description: item.Description, Status: item.Status}
	if item.Status == "cancelled" {
		return event, nil
	}
	if event.Title == "" {
		event.Title = "Busy"
	}
	event.Timezone = item.Start.Timezone
	if event.Timezone == "" {
		event.Timezone = calendarTimezone
	}
	if item.Start.Date != "" || item.End.Date != "" {
		if item.Start.Date == "" || item.End.Date == "" || event.Timezone == "" {
			return services.ProviderEvent{}, errors.New("Google Calendar all-day event is invalid")
		}
		startDate, startDateError := time.Parse(time.DateOnly, item.Start.Date)
		endDate, endDateError := time.Parse(time.DateOnly, item.End.Date)
		if startDateError != nil || endDateError != nil || endDate.Before(startDate) {
			return services.ProviderEvent{}, errors.New("Google Calendar all-day event is invalid")
		}
		if endDate.Equal(startDate) {
			endDate = startDate.AddDate(0, 0, 1)
		}
		event.StartDate, event.EndDate = startDate.Format(time.DateOnly), endDate.Format(time.DateOnly)
		return event, nil
	}
	start, startError := time.Parse(time.RFC3339, item.Start.DateTime)
	end, endError := time.Parse(time.RFC3339, item.End.DateTime)
	if startError != nil || endError != nil || end.Before(start) || event.Timezone == "" {
		return services.ProviderEvent{}, errors.New("Google Calendar timed event is invalid")
	}
	canonicalStart := start.UTC()
	if end.Equal(start) {
		event.At = &canonicalStart
		return event, nil
	}
	canonicalEnd := end.UTC()
	event.StartsAt, event.EndsAt = &canonicalStart, &canonicalEnd
	return event, nil
}
