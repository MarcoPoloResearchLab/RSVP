// Package config loads the canonical backend configuration.
package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// DatabaseConfig selects the SQLite database.
type DatabaseConfig struct {
	Name string `yaml:"name"`
}

// ApplicationContext contains the shared application dependencies.
type ApplicationContext struct {
	Database          *gorm.DB
	Logger            *log.Logger
	AppBaseURL        string
	WebsiteURL        string
	CalendarReturnURL string
}

// AuthConfig defines the TAuth authorization and browser contract.
type AuthConfig struct {
	SigningKey  string `yaml:"jwt_signing_key"`
	Issuer      string `yaml:"jwt_issuer"`
	CookieName  string `yaml:"cookie_name"`
	TenantID    string `yaml:"tenant_id"`
	URL         string `yaml:"url"`
	UpstreamURL string `yaml:"upstream_url"`
	LoginPath   string `yaml:"login_path"`
	LogoutPath  string `yaml:"logout_path"`
	NoncePath   string `yaml:"nonce_path"`
	SessionPath string `yaml:"session_path"`
}

// LLMProxyConfig defines the official client routing and work budget.
type LLMProxyConfig struct {
	BaseURL               string  `yaml:"base_url"`
	Secret                string  `yaml:"secret"`
	Provider              *string `yaml:"provider"`
	Model                 *string `yaml:"model"`
	ReasoningEffort       string  `yaml:"reasoning_effort"`
	RequestTimeoutSeconds int     `yaml:"request_timeout_seconds"`
}

// EnvConfig is the canonical, startup-validated backend configuration.
type EnvConfig struct {
	Server struct {
		Address string `yaml:"address"`
	} `yaml:"server"`
	Database                            DatabaseConfig `yaml:"database"`
	AppBaseURL                          string         `yaml:"app_base_url"`
	WebsiteURL                          string         `yaml:"website_url"`
	Auth                                AuthConfig     `yaml:"tauth"`
	LLMProxy                            LLMProxyConfig `yaml:"llm_proxy"`
	CalendarCredentialEncryptionKey     string         `yaml:"calendar_credential_encryption_key"`
	GoogleClientID                      string         `yaml:"google_client_id"`
	GoogleClientSecret                  string         `yaml:"google_client_secret"`
	GoogleCalendarAuthorizationEndpoint string         `yaml:"google_calendar_authorization_endpoint"`
	GoogleCalendarTokenEndpoint         string         `yaml:"google_calendar_token_endpoint"`
	GoogleCalendarListEndpoint          string         `yaml:"google_calendar_list_endpoint"`
	GoogleCalendarEventsEndpoint        string         `yaml:"google_calendar_events_endpoint"`
}

// Load reads the strict YAML schema and resolves its environment references.
func Load(path string) (*EnvConfig, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read backend configuration %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(payload))
	decoder.KnownFields(true)
	var configuration EnvConfig
	if err := decoder.Decode(&configuration); err != nil {
		return nil, fmt.Errorf("decode backend configuration %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("backend configuration %s must contain one document", path)
	}
	if err := expandReferences(reflect.ValueOf(&configuration).Elem()); err != nil {
		return nil, fmt.Errorf("resolve backend configuration %s: %w", path, err)
	}
	if err := configuration.Validate(); err != nil {
		return nil, fmt.Errorf("validate backend configuration %s: %w", path, err)
	}
	return &configuration, nil
}

func expandReferences(value reflect.Value) error {
	switch value.Kind() {
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			if err := expandReferences(value.Field(index)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if !value.IsNil() {
			return expandReferences(value.Elem())
		}
	case reflect.String:
		var missing string
		expanded := os.Expand(value.String(), func(name string) string {
			text, found := os.LookupEnv(name)
			if !found || text == "" {
				missing = name
			}
			return text
		})
		if missing != "" {
			return fmt.Errorf("required environment variable %s is absent or empty", missing)
		}
		value.SetString(expanded)
	}
	return nil
}

// Validate checks external configuration before startup constructs clients.
func (configuration EnvConfig) Validate() error {
	required := map[string]string{
		"server.address": configuration.Server.Address, "database.name": configuration.Database.Name,
		"tauth.jwt_signing_key": configuration.Auth.SigningKey, "tauth.cookie_name": configuration.Auth.CookieName,
		"tauth.tenant_id": configuration.Auth.TenantID, "google_client_id": configuration.GoogleClientID,
		"google_client_secret": configuration.GoogleClientSecret, "llm_proxy.secret": configuration.LLMProxy.Secret,
		"llm_proxy.reasoning_effort": configuration.LLMProxy.ReasoningEffort,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if _, _, err := net.SplitHostPort(configuration.Server.Address); err != nil {
		return errors.New("server.address must contain a host and port")
	}
	if configuration.Auth.Issuer != "tauth" {
		return errors.New("tauth.jwt_issuer must be tauth")
	}
	if len(configuration.Auth.SigningKey) < 32 {
		return errors.New("tauth.jwt_signing_key must contain at least 32 bytes")
	}
	if configuration.LLMProxy.Provider == nil || configuration.LLMProxy.Model == nil || configuration.LLMProxy.RequestTimeoutSeconds <= 0 {
		return errors.New("llm_proxy requires explicit provider, model, and a positive request_timeout_seconds")
	}
	key, err := base64.StdEncoding.DecodeString(configuration.CalendarCredentialEncryptionKey)
	if err != nil || len(key) != 32 {
		return errors.New("calendar_credential_encryption_key must contain 32 base64-encoded bytes")
	}
	urls := map[string]string{
		"app_base_url": configuration.AppBaseURL, "website_url": configuration.WebsiteURL, "llm_proxy.base_url": configuration.LLMProxy.BaseURL,
		"tauth.upstream_url":                     configuration.Auth.UpstreamURL,
		"tauth.url":                              configuration.Auth.URL,
		"google_calendar_authorization_endpoint": configuration.GoogleCalendarAuthorizationEndpoint,
		"google_calendar_token_endpoint":         configuration.GoogleCalendarTokenEndpoint,
		"google_calendar_list_endpoint":          configuration.GoogleCalendarListEndpoint,
		"google_calendar_events_endpoint":        configuration.GoogleCalendarEventsEndpoint,
	}
	for name, address := range urls {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("%s must be an HTTP or HTTPS URL without credentials, query, or fragment", name)
		}
	}
	for name, path := range map[string]string{"login_path": configuration.Auth.LoginPath, "logout_path": configuration.Auth.LogoutPath, "nonce_path": configuration.Auth.NoncePath, "session_path": configuration.Auth.SessionPath} {
		if !strings.HasPrefix(path, "/auth/") || strings.ContainsAny(path, "?#") {
			return fmt.Errorf("tauth.%s must be an explicit /auth/ path", name)
		}
	}
	if !strings.HasSuffix(configuration.AppBaseURL, "/") {
		return errors.New("app_base_url must end with a slash")
	}
	return nil
}
