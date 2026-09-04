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

// ProviderCalendar contains one provider-owned source calendar change.
type ProviderCalendar struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	ColorToken          string                 `json:"color_token"`
	Deleted             bool                   `json:"deleted"`
	Readable            bool                   `json:"readable"`
	Visible             bool                   `json:"visible"`
	Default             bool                   `json:"default"`
	SemanticSourceGroup *SemanticCalendarGroup `json:"semantic_source_group,omitempty"`
}

// ProviderCalendarBatch contains all CalendarList pages for one cursor transition.
type ProviderCalendarBatch struct {
	Calendars      []ProviderCalendar
	NextSyncCursor string
}

// ProviderEventChange contains one normalized provider event mutation.
type ProviderEventChange struct {
	ProviderEventID  string
	ProviderSeriesID string
	SemanticGroup    SemanticCalendarGroup
	Deleted          bool
	DiagnosticCode   string
	Title            string
	Description      string
	Timezone         string
	At               *time.Time
	StartsAt         *time.Time
	EndsAt           *time.Time
	StartDate        string
	EndDate          string
}

// ProviderEventBatch contains all pages for one cursor transition.
type ProviderEventBatch struct {
	Changes        []ProviderEventChange
	NextSyncCursor string
}

// SemanticCalendarGroup identifies one provider-neutral event grouping.
type SemanticCalendarGroup string

const (
	// SemanticCalendarGroupCalendar identifies the related provider calendar.
	SemanticCalendarGroupCalendar SemanticCalendarGroup = "calendar"
	// SemanticCalendarGroupBirthdays identifies normalized birthday meaning.
	SemanticCalendarGroupBirthdays SemanticCalendarGroup = "birthdays"
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
	SynchronizeEvents(ctx context.Context, credential CalendarProviderCredential, providerCalendarID string, syncCursor string) (ProviderEventBatch, error)
}
