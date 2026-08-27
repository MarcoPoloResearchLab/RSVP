package services

import (
	"context"
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

// CalendarProviderAdapter defines the external consent and calendar-list boundary.
type CalendarProviderAdapter interface {
	AuthorizationURL(state string, redirectURI string) (string, error)
	ExchangeCode(ctx context.Context, code string, redirectURI string) (CalendarProviderCredential, error)
	ListCalendars(ctx context.Context, credential CalendarProviderCredential) ([]ProviderCalendar, error)
}
