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

// ProviderCalendarGroup contains one provider-normalized event grouping.
type ProviderCalendarGroup struct {
	Key        ProviderCalendarGroupKey `json:"key"`
	Name       string                   `json:"name"`
	ColorToken string                   `json:"color_token"`
	Visible    bool                     `json:"visible"`
}

// ProviderCalendar contains one provider-owned source calendar change.
type ProviderCalendar struct {
	ID       string                  `json:"id"`
	Deleted  bool                    `json:"deleted"`
	Readable bool                    `json:"readable"`
	Groups   []ProviderCalendarGroup `json:"groups"`
}

// ProviderCalendarBatch contains all CalendarList pages for one cursor transition.
type ProviderCalendarBatch struct {
	Calendars      []ProviderCalendar
	NextSyncCursor string
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

// ProviderCalendarGroupKey identifies one semantic provider event grouping.
type ProviderCalendarGroupKey string

const (
	// ProviderCalendarGroupCalendar identifies the provider calendar's general events.
	ProviderCalendarGroupCalendar ProviderCalendarGroupKey = "calendar"
	// ProviderCalendarGroupBirthdays identifies birthday events.
	ProviderCalendarGroupBirthdays ProviderCalendarGroupKey = "birthdays"
)

var (
	// ErrCalendarListSyncCursorRejected indicates that the provider requires a complete source calendar reconciliation.
	ErrCalendarListSyncCursorRejected = errors.New("calendar list sync cursor was rejected")
	// ErrCalendarSyncCursorRejected indicates that the provider requires a complete event reconciliation.
	ErrCalendarSyncCursorRejected = errors.New("calendar sync cursor was rejected")
)

// CalendarProviderAdapter defines the external calendar provider boundary.
type CalendarProviderAdapter interface {
	AuthorizationURL(state string, redirectURI string) (string, error)
	ExchangeCode(ctx context.Context, code string, redirectURI string) (CalendarProviderCredential, error)
	RefreshCredential(ctx context.Context, credential CalendarProviderCredential) (CalendarProviderCredential, error)
	ListCalendars(ctx context.Context, credential CalendarProviderCredential, syncCursor string) (ProviderCalendarBatch, error)
	SynchronizeEvents(ctx context.Context, credential CalendarProviderCredential, providerCalendarID string, providerGroup ProviderCalendarGroupKey, syncCursor string) (ProviderEventBatch, error)
}
