package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// CalendarProvider identifies one external calendar provider.
type CalendarProvider string

const (
	// CalendarProviderGoogle identifies Google Calendar.
	CalendarProviderGoogle CalendarProvider = "google"
)

// CalendarConnectionStatus identifies one closed connection state.
type CalendarConnectionStatus string

// SourceCalendarGroup identifies one normalized provider event grouping.
type SourceCalendarGroup string

const (
	// CalendarConnectionConnected identifies an authorized connection.
	CalendarConnectionConnected CalendarConnectionStatus = "connected"
	// SourceCalendarGroupCalendar identifies the provider calendar's general event grouping.
	SourceCalendarGroupCalendar SourceCalendarGroup = "calendar"
	// SourceCalendarGroupBirthdays identifies birthday events.
	SourceCalendarGroupBirthdays SourceCalendarGroup = "birthdays"
)

var (
	ErrCalendarProviderInvalid      = errors.New("calendar provider is invalid")
	ErrAuthorizationRequestInvalid  = errors.New("calendar authorization request is invalid")
	ErrCalendarConnectionInvalid    = errors.New("calendar connection is invalid")
	ErrSourceCalendarMappingInvalid = errors.New("source calendar mapping is invalid")
)

// CalendarAuthorizationRequest stores one expiring consent state hash.
type CalendarAuthorizationRequest struct {
	BaseModel
	OrganizerID string           `gorm:"type:varchar(8);not null;index"`
	Provider    CalendarProvider `gorm:"type:text;not null;check:calendar_authorization_provider,provider = 'google'"`
	StateHash   []byte           `gorm:"not null;uniqueIndex"`
	RedirectURI string           `gorm:"not null"`
	ExpiresAt   time.Time        `gorm:"not null"`
	UsedAt      *time.Time
	Organizer   User `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewCalendarAuthorizationRequest constructs one valid consent request.
func NewCalendarAuthorizationRequest(organizerID string, stateHash []byte, redirectURI string, expiresAt time.Time) (*CalendarAuthorizationRequest, error) {
	request := &CalendarAuthorizationRequest{OrganizerID: organizerID, Provider: CalendarProviderGoogle, StateHash: append([]byte(nil), stateHash...), RedirectURI: redirectURI, ExpiresAt: expiresAt.UTC()}
	if validationError := request.Validate(); validationError != nil {
		return nil, validationError
	}
	return request, nil
}

func (request *CalendarAuthorizationRequest) Validate() error {
	if request.OrganizerID == "" || request.Provider != CalendarProviderGoogle || len(request.StateHash) != 32 || request.RedirectURI == "" || request.ExpiresAt.IsZero() {
		return ErrAuthorizationRequestInvalid
	}
	return nil
}

func (request *CalendarAuthorizationRequest) BeforeCreate(database *gorm.DB) error {
	if validationError := request.Validate(); validationError != nil {
		return validationError
	}
	return request.BaseModel.GenerateID(database, request)
}
func (request *CalendarAuthorizationRequest) BeforeUpdate(*gorm.DB) error {
	return request.Validate()
}
func (request *CalendarAuthorizationRequest) GetTableName() string {
	return config.TableCalendarAuthorizationRequests
}
func (request *CalendarAuthorizationRequest) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// CalendarConnection stores one encrypted calendar provider credential.
type CalendarConnection struct {
	BaseModel
	OrganizerID             string                   `gorm:"type:varchar(8);not null;uniqueIndex:calendar_connection_owner"`
	Provider                CalendarProvider         `gorm:"type:text;not null;uniqueIndex:calendar_connection_owner;check:calendar_connection_provider,provider = 'google'"`
	CredentialNonce         []byte                   `gorm:"not null"`
	CredentialCiphertext    []byte                   `gorm:"not null"`
	Status                  CalendarConnectionStatus `gorm:"type:text;not null;check:calendar_connection_status,status = 'connected'"`
	CalendarListSyncCursor  *string
	CalendarImportCutoverAt *time.Time
	Organizer               User                    `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Mappings                []SourceCalendarMapping `gorm:"foreignKey:ConnectionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewCalendarConnection constructs one encrypted Google Calendar connection.
func NewCalendarConnection(organizerID string, nonce []byte, ciphertext []byte) (*CalendarConnection, error) {
	connection := &CalendarConnection{OrganizerID: organizerID, Provider: CalendarProviderGoogle, CredentialNonce: append([]byte(nil), nonce...), CredentialCiphertext: append([]byte(nil), ciphertext...), Status: CalendarConnectionConnected}
	if validationError := connection.Validate(); validationError != nil {
		return nil, validationError
	}
	return connection, nil
}

func (connection *CalendarConnection) Validate() error {
	if connection.OrganizerID == "" || connection.Provider != CalendarProviderGoogle || len(connection.CredentialNonce) != 12 || len(connection.CredentialCiphertext) == 0 || connection.Status != CalendarConnectionConnected {
		return ErrCalendarConnectionInvalid
	}
	return nil
}

func (connection *CalendarConnection) BeforeCreate(database *gorm.DB) error {
	if validationError := connection.Validate(); validationError != nil {
		return validationError
	}
	return connection.BaseModel.GenerateID(database, connection)
}
func (connection *CalendarConnection) BeforeUpdate(*gorm.DB) error { return connection.Validate() }
func (connection *CalendarConnection) GetTableName() string        { return config.TableCalendarConnections }
func (connection *CalendarConnection) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// SourceCalendarMapping connects one semantic calendar group to one RSVP calendar.
type SourceCalendarMapping struct {
	BaseModel
	ConnectionID       string              `gorm:"type:varchar(8);not null;uniqueIndex:source_provider_calendar,priority:1"`
	CalendarID         string              `gorm:"type:varchar(8);not null;uniqueIndex"`
	ProviderCalendarID string              `gorm:"not null;uniqueIndex:source_provider_calendar,priority:2"`
	SemanticGroup      SourceCalendarGroup `gorm:"column:semantic_group;type:text;not null;default:calendar;uniqueIndex:source_provider_calendar,priority:3;check:source_calendar_group,semantic_group IN ('calendar','birthdays')"`
	SyncCursor         *string
	Connection         CalendarConnection `gorm:"foreignKey:ConnectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Calendar           Calendar           `gorm:"foreignKey:CalendarID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// NewSourceCalendarMapping constructs one source calendar mapping.
func NewSourceCalendarMapping(connectionID string, calendarID string, providerCalendarID string, providerGroup SourceCalendarGroup) (*SourceCalendarMapping, error) {
	mapping := &SourceCalendarMapping{ConnectionID: connectionID, CalendarID: calendarID, ProviderCalendarID: providerCalendarID, SemanticGroup: providerGroup}
	if validationError := mapping.Validate(); validationError != nil {
		return nil, validationError
	}
	return mapping, nil
}

func (mapping *SourceCalendarMapping) Validate() error {
	if mapping.ConnectionID == "" || mapping.CalendarID == "" || mapping.ProviderCalendarID == "" ||
		(mapping.SemanticGroup != SourceCalendarGroupCalendar && mapping.SemanticGroup != SourceCalendarGroupBirthdays) {
		return ErrSourceCalendarMappingInvalid
	}
	return nil
}

func (mapping *SourceCalendarMapping) BeforeCreate(database *gorm.DB) error {
	if validationError := mapping.Validate(); validationError != nil {
		return validationError
	}
	return mapping.BaseModel.GenerateID(database, mapping)
}
func (mapping *SourceCalendarMapping) BeforeUpdate(*gorm.DB) error { return mapping.Validate() }
func (mapping *SourceCalendarMapping) GetTableName() string {
	return config.TableSourceCalendarMappings
}
func (mapping *SourceCalendarMapping) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}
