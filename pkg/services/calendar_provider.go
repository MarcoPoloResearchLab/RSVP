package services

import (
	"context"
	"errors"
	"time"
)

// CalendarProviderCredential contains one provider access grant before encryption.
type CalendarProviderCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ProviderCalendar contains one source calendar available for selection.
type ProviderCalendar struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Timezone   string `json:"timezone"`
	ColorToken string `json:"color_token"`
}

// ProviderEvent contains one provider-owned event occurrence or deletion.
type ProviderEvent struct {
	ID          string
	SeriesID    string
	Title       string
	Description string
	Timezone    string
	Status      string
	At          *time.Time
	StartsAt    *time.Time
	EndsAt      *time.Time
	StartDate   string
	EndDate     string
}

// ProviderEventBatch contains all pages for one cursor transition.
type ProviderEventBatch struct {
	Events         []ProviderEvent
	NextSyncCursor string
}

// ErrCalendarSyncCursorRejected indicates that the provider requires a complete reconciliation.
var ErrCalendarSyncCursorRejected = errors.New("calendar sync cursor was rejected")

// CalendarProviderAdapter defines the external calendar provider boundary.
type CalendarProviderAdapter interface {
	AuthorizationURL(state string, redirectURI string) (string, error)
	ExchangeCode(ctx context.Context, code string, redirectURI string) (CalendarProviderCredential, error)
	RefreshCredential(ctx context.Context, credential CalendarProviderCredential) (CalendarProviderCredential, error)
	ListCalendars(ctx context.Context, credential CalendarProviderCredential) ([]ProviderCalendar, error)
	SynchronizeEvents(ctx context.Context, credential CalendarProviderCredential, providerCalendarID string, syncCursor string) (ProviderEventBatch, error)
}
