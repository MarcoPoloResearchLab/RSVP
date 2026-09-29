package routes_test

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/tyemirov/tauth/pkg/sessionvalidator"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/routetestfixture"
	"github.com/tyemirov/RSVP/internal/testsupport"
)

func TestI005PublicUIConfigAndResourceAuthorization(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	mux := http.NewServeMux()
	routetestfixture.New(t, fixture.ApplicationContext).RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, scenario := range []struct {
		path    string
		status  int
		content string
	}{
		{"/config-ui.yaml", http.StatusOK, "tenantId: rsvp-development"},
		{"/horizon/", http.StatusUnauthorized, "authentication_required"},
		{"/calendar-connections/", http.StatusUnauthorized, "authentication_required"},
		{"/calendars/", http.StatusUnauthorized, "authentication_required"},
		{"/lanes/", http.StatusUnauthorized, "authentication_required"},
		{"/organizers/current", http.StatusUnauthorized, "authentication_required"},
		{"/events/", http.StatusUnauthorized, "authentication_required"},
		{"/venues/", http.StatusUnauthorized, "authentication_required"},
		{"/rsvps/", http.StatusUnauthorized, "authentication_required"},
		{"/probes/", http.StatusUnauthorized, "authentication_required"},
		{"/attention-policies/", http.StatusUnauthorized, "authentication_required"},
		{"/derived-marker-rules/", http.StatusUnauthorized, "authentication_required"},
		{"/ingestion-drafts/", http.StatusUnauthorized, "authentication_required"},
		{"/calendar-authorization-requests/", http.StatusUnauthorized, "authentication_required"},
		{"/healthz", http.StatusOK, "OK"},
	} {
		t.Run(scenario.path, func(t *testing.T) {
			response, err := client.Get(server.URL + scenario.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != scenario.status || !strings.Contains(string(body), scenario.content) {
				t.Fatalf("GET %s: status %d, expected %d; required content absent: %t", scenario.path, response.StatusCode, scenario.status, !strings.Contains(string(body), scenario.content))
			}
		})
	}
}

func TestTAuthCookieBoundaryRejectsForeignExpiredAndUnsignedIdentity(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	mux := http.NewServeMux()
	routetestfixture.New(t, fixture.ApplicationContext).RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	configuration := testsupport.EnvironmentConfig()
	for _, scenario := range []struct {
		name, tenant string
		expiry       *jwt.NumericDate
		signingKey   string
	}{
		{"foreign tenant", "another-tenant", jwt.NewNumericDate(time.Now().Add(time.Hour)), configuration.Auth.SigningKey},
		{"expired", configuration.Auth.TenantID, jwt.NewNumericDate(time.Now().Add(-time.Hour)), configuration.Auth.SigningKey},
		{"missing expiry", configuration.Auth.TenantID, nil, configuration.Auth.SigningKey},
		{"untrusted signer", configuration.Auth.TenantID, jwt.NewNumericDate(time.Now().Add(time.Hour)), "another-key"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			claims := sessionvalidator.Claims{TenantID: scenario.tenant, UserID: owner.ID, UserEmail: owner.Email, RegisteredClaims: jwt.RegisteredClaims{Issuer: "tauth", ExpiresAt: scenario.expiry}}
			value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(scenario.signingKey))
			if err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequest(http.MethodGet, server.URL+"/calendars/", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.AddCookie(&http.Cookie{Name: configuration.Auth.CookieName, Value: value})
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("authorization status=%d", response.StatusCode)
			}
		})
	}
	response, err := server.Client().Get(server.URL + "/config-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{configuration.Auth.SigningKey, configuration.LLMProxy.Secret, configuration.GoogleClientSecret, configuration.CalendarCredentialEncryptionKey} {
		if strings.Contains(string(payload), secret) {
			t.Fatal("The browser configuration contains a private value")
		}
	}
}
