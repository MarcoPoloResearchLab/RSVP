package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	calendarAuthorizationLifetime     = 10 * time.Minute
	idempotencyLifetime               = 24 * time.Hour
	createCalendarConnectionOperation = "create_calendar_connection"
)

var (
	// ErrCredentialEncryptionKeyInvalid indicates that the private key does not contain 32 bytes.
	ErrCredentialEncryptionKeyInvalid = errors.New("calendar credential encryption key is invalid")
	// ErrCalendarAuthorizationInvalid indicates that consent confirmation does not match one live request.
	ErrCalendarAuthorizationInvalid = errors.New("calendar authorization confirmation is invalid")
	// ErrIdempotencyKeyRequired indicates that retry-safe creation has no request key.
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	// ErrIdempotencyConflict indicates that a key identifies a different request.
	ErrIdempotencyConflict = errors.New("idempotency key identifies a different request")
	// ErrSourceCalendarSelectionInvalid indicates that a selected provider calendar is unavailable.
	ErrSourceCalendarSelectionInvalid = errors.New("source calendar selection is invalid")
)

// CredentialCipher encrypts calendar credentials with AES-256-GCM.
type CredentialCipher struct {
	aead   cipher.AEAD
	random io.Reader
}

// NewCredentialCipher constructs one credential cipher from a base64 key.
func NewCredentialCipher(encodedKey string, random io.Reader) (*CredentialCipher, error) {
	key, decodeError := base64.StdEncoding.DecodeString(encodedKey)
	if decodeError != nil || len(key) != 32 || random == nil {
		return nil, ErrCredentialEncryptionKeyInvalid
	}
	block, blockError := aes.NewCipher(key)
	if blockError != nil {
		return nil, fmt.Errorf("construct calendar credential cipher: %w", blockError)
	}
	aead, aeadError := cipher.NewGCM(block)
	if aeadError != nil {
		return nil, fmt.Errorf("construct calendar credential AEAD: %w", aeadError)
	}
	return &CredentialCipher{aead: aead, random: random}, nil
}

func (credentialCipher *CredentialCipher) encrypt(credential CalendarProviderCredential) ([]byte, []byte, error) {
	payload, marshalError := json.Marshal(credential)
	if marshalError != nil {
		return nil, nil, fmt.Errorf("serialize calendar credential: %w", marshalError)
	}
	nonce := make([]byte, credentialCipher.aead.NonceSize())
	if _, randomError := io.ReadFull(credentialCipher.random, nonce); randomError != nil {
		return nil, nil, fmt.Errorf("create calendar credential nonce: %w", randomError)
	}
	ciphertext := credentialCipher.aead.Seal(nil, nonce, payload, nil)
	return nonce, ciphertext, nil
}

func (credentialCipher *CredentialCipher) decrypt(nonce []byte, ciphertext []byte) (CalendarProviderCredential, error) {
	payload, openError := credentialCipher.aead.Open(nil, nonce, ciphertext, nil)
	if openError != nil {
		return CalendarProviderCredential{}, errors.New("decrypt calendar credential")
	}
	var credential CalendarProviderCredential
	if unmarshalError := json.Unmarshal(payload, &credential); unmarshalError != nil {
		return CalendarProviderCredential{}, errors.New("decode calendar credential")
	}
	if credential.AccessToken == "" || credential.RefreshToken == "" || credential.ExpiresAt.IsZero() {
		return CalendarProviderCredential{}, errors.New("stored calendar credential is invalid")
	}
	return credential, nil
}

// AuthorizationStart contains one persisted request and its external consent URL.
type AuthorizationStart struct {
	RequestID        string `json:"request_id"`
	AuthorizationURL string `json:"authorization_url"`
}

// AuthorizationConfirmation contains callback data that has not changed persistence.
type AuthorizationConfirmation struct {
	RequestID string `json:"request_id"`
	State     string `json:"state"`
	Code      string `json:"code"`
}

// CalendarConnectionService owns Google Calendar consent and source selection.
type CalendarConnectionService struct {
	database *gorm.DB
	adapter  CalendarProviderAdapter
	cipher   *CredentialCipher
	now      func() time.Time
	random   io.Reader
}

// NewCalendarConnectionService constructs one calendar connection service.
func NewCalendarConnectionService(database *gorm.DB, adapter CalendarProviderAdapter, credentialCipher *CredentialCipher, now func() time.Time, random io.Reader) (*CalendarConnectionService, error) {
	if database == nil || adapter == nil || credentialCipher == nil || now == nil || random == nil {
		return nil, errors.New("calendar connection dependencies are required")
	}
	return &CalendarConnectionService{database: database, adapter: adapter, cipher: credentialCipher, now: now, random: random}, nil
}

// CreateAuthorizationRequest creates one expiring state hash and consent URL.
func (service *CalendarConnectionService) CreateAuthorizationRequest(ctx context.Context, organizerID string, redirectURI string) (AuthorizationStart, error) {
	stateBytes := make([]byte, 32)
	if _, randomError := io.ReadFull(service.random, stateBytes); randomError != nil {
		return AuthorizationStart{}, fmt.Errorf("create calendar authorization state: %w", randomError)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	authorizationURL, authorizationError := service.adapter.AuthorizationURL(state, redirectURI)
	if authorizationError != nil {
		return AuthorizationStart{}, authorizationError
	}
	stateHash := sha256.Sum256([]byte(state))
	requestRecord, requestError := models.NewCalendarAuthorizationRequest(organizerID, stateHash[:], redirectURI, service.now().UTC().Add(calendarAuthorizationLifetime))
	if requestError != nil {
		return AuthorizationStart{}, requestError
	}
	if createError := service.database.WithContext(ctx).Create(requestRecord).Error; createError != nil {
		return AuthorizationStart{}, fmt.Errorf("create calendar authorization request for organizer %s: %w", organizerID, createError)
	}
	return AuthorizationStart{RequestID: requestRecord.ID, AuthorizationURL: authorizationURL}, nil
}

// ValidateCallback validates consent callback data without a database change.
func (service *CalendarConnectionService) ValidateCallback(ctx context.Context, organizerID string, state string, code string) (AuthorizationConfirmation, error) {
	if state == "" || code == "" {
		return AuthorizationConfirmation{}, ErrCalendarAuthorizationInvalid
	}
	stateHash := sha256.Sum256([]byte(state))
	var requestRecord models.CalendarAuthorizationRequest
	if findError := service.database.WithContext(ctx).First(&requestRecord, "organizer_id = ? AND state_hash = ?", organizerID, stateHash[:]).Error; findError != nil {
		return AuthorizationConfirmation{}, ErrCalendarAuthorizationInvalid
	}
	if requestRecord.UsedAt != nil || !requestRecord.ExpiresAt.After(service.now().UTC()) {
		return AuthorizationConfirmation{}, ErrCalendarAuthorizationInvalid
	}
	return AuthorizationConfirmation{RequestID: requestRecord.ID, State: state, Code: code}, nil
}

// CreateConnection exchanges consent and stores only encrypted credential data.
func (service *CalendarConnectionService) CreateConnection(ctx context.Context, organizerID string, confirmation AuthorizationConfirmation, idempotencyKey string) (*models.CalendarConnection, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, ErrIdempotencyKeyRequired
	}
	requestPayload, marshalError := json.Marshal(confirmation)
	if marshalError != nil {
		return nil, false, fmt.Errorf("serialize connection request: %w", marshalError)
	}
	keyHash := sha256.Sum256([]byte(idempotencyKey))
	requestHash := sha256.Sum256(requestPayload)
	if existing, found, lookupError := service.readIdempotentConnection(ctx, organizerID, keyHash[:], requestHash[:]); lookupError != nil || found {
		return existing, found, lookupError
	}
	requestRecord, validationError := service.authorizationRequest(ctx, organizerID, confirmation)
	if validationError != nil {
		return nil, false, validationError
	}
	credential, exchangeError := service.adapter.ExchangeCode(ctx, confirmation.Code, requestRecord.RedirectURI)
	if exchangeError != nil {
		return nil, false, exchangeError
	}
	nonce, ciphertext, encryptionError := service.cipher.encrypt(credential)
	if encryptionError != nil {
		return nil, false, encryptionError
	}
	var createdConnection *models.CalendarConnection
	reusedConnection := false
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if existing, found, lookupError := service.readIdempotentConnectionWithDatabase(transaction, organizerID, keyHash[:], requestHash[:]); lookupError != nil || found {
			createdConnection = existing
			reusedConnection = found
			return lookupError
		}
		var lockedRequest models.CalendarAuthorizationRequest
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedRequest, "id = ? AND organizer_id = ?", confirmation.RequestID, organizerID).Error; findError != nil {
			return ErrCalendarAuthorizationInvalid
		}
		stateHash := sha256.Sum256([]byte(confirmation.State))
		if lockedRequest.UsedAt != nil || !lockedRequest.ExpiresAt.After(service.now().UTC()) || subtle.ConstantTimeCompare(lockedRequest.StateHash, stateHash[:]) != 1 {
			return ErrCalendarAuthorizationInvalid
		}
		connection, connectionError := models.NewCalendarConnection(organizerID, nonce, ciphertext)
		if connectionError != nil {
			return connectionError
		}
		if createError := transaction.Create(connection).Error; createError != nil {
			return fmt.Errorf("create Google Calendar connection for organizer %s: %w", organizerID, createError)
		}
		usedAt := service.now().UTC()
		if updateError := transaction.Model(&lockedRequest).Update("used_at", usedAt).Error; updateError != nil {
			return fmt.Errorf("use calendar authorization request %s: %w", lockedRequest.ID, updateError)
		}
		record, recordError := models.NewIdempotencyRecord(organizerID, createCalendarConnectionOperation, keyHash[:], requestHash[:], http.StatusCreated, "calendar_connection", connection.ID, service.now().UTC().Add(idempotencyLifetime))
		if recordError != nil {
			return recordError
		}
		if createError := transaction.Create(record).Error; createError != nil {
			return fmt.Errorf("create connection idempotency record: %w", createError)
		}
		createdConnection = connection
		return nil
	})
	return createdConnection, reusedConnection, transactionError
}

func (service *CalendarConnectionService) authorizationRequest(ctx context.Context, organizerID string, confirmation AuthorizationConfirmation) (*models.CalendarAuthorizationRequest, error) {
	stateHash := sha256.Sum256([]byte(confirmation.State))
	var requestRecord models.CalendarAuthorizationRequest
	if findError := service.database.WithContext(ctx).First(&requestRecord, "id = ? AND organizer_id = ?", confirmation.RequestID, organizerID).Error; findError != nil {
		return nil, ErrCalendarAuthorizationInvalid
	}
	if confirmation.Code == "" || requestRecord.UsedAt != nil || !requestRecord.ExpiresAt.After(service.now().UTC()) || subtle.ConstantTimeCompare(requestRecord.StateHash, stateHash[:]) != 1 {
		return nil, ErrCalendarAuthorizationInvalid
	}
	return &requestRecord, nil
}

// ReadConnection returns one organizer-owned connection and its mappings.
func (service *CalendarConnectionService) ReadConnection(ctx context.Context, organizerID string, connectionID string) (*models.CalendarConnection, error) {
	var connection models.CalendarConnection
	if findError := service.database.WithContext(ctx).Preload("Mappings").First(&connection, "id = ?", connectionID).Error; findError != nil {
		return nil, findError
	}
	if connection.OrganizerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &connection, nil
}

// ListSourceCalendars returns provider calendars without exposing credentials.
func (service *CalendarConnectionService) ListSourceCalendars(ctx context.Context, organizerID string, connectionID string) ([]ProviderCalendar, error) {
	connection, connectionError := service.ReadConnection(ctx, organizerID, connectionID)
	if connectionError != nil {
		return nil, connectionError
	}
	credential, decryptionError := service.cipher.decrypt(connection.CredentialNonce, connection.CredentialCiphertext)
	if decryptionError != nil {
		return nil, decryptionError
	}
	return service.adapter.ListCalendars(ctx, credential)
}

// ReplaceSourceCalendars replaces the selected source calendar mappings.
func (service *CalendarConnectionService) ReplaceSourceCalendars(ctx context.Context, organizer *models.User, timezone models.Timezone, connectionID string, providerCalendarIDs []string) ([]models.SourceCalendarMapping, error) {
	availableCalendars, listError := service.ListSourceCalendars(ctx, organizer.ID, connectionID)
	if listError != nil {
		return nil, listError
	}
	availableByID := make(map[string]ProviderCalendar, len(availableCalendars))
	for _, calendar := range availableCalendars {
		availableByID[calendar.ID] = calendar
	}
	selectedIDs := make(map[string]struct{}, len(providerCalendarIDs))
	for _, providerCalendarID := range providerCalendarIDs {
		if _, found := availableByID[providerCalendarID]; !found {
			return nil, ErrSourceCalendarSelectionInvalid
		}
		if _, duplicate := selectedIDs[providerCalendarID]; duplicate {
			return nil, ErrSourceCalendarSelectionInvalid
		}
		selectedIDs[providerCalendarID] = struct{}{}
	}
	var mappings []models.SourceCalendarMapping
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if confirmationError := organizer.ConfirmTimezone(transaction, timezone); confirmationError != nil {
			return confirmationError
		}
		var connection models.CalendarConnection
		if findError := transaction.First(&connection, "id = ?", connectionID).Error; findError != nil {
			return findError
		}
		if connection.OrganizerID != organizer.ID {
			return ErrResourceForbidden
		}
		var existing []models.SourceCalendarMapping
		if findError := transaction.Where("connection_id = ?", connectionID).Find(&existing).Error; findError != nil {
			return findError
		}
		existingByProviderID := make(map[string]models.SourceCalendarMapping, len(existing))
		for _, mapping := range existing {
			existingByProviderID[mapping.ProviderCalendarID] = mapping
			if _, selected := selectedIDs[mapping.ProviderCalendarID]; selected {
				continue
			}
			if deleteError := transaction.Unscoped().Delete(&mapping).Error; deleteError != nil {
				return fmt.Errorf("delete source calendar mapping %s: %w", mapping.ID, deleteError)
			}
			if deleteError := transaction.Unscoped().Delete(&models.Calendar{}, "id = ?", mapping.CalendarID).Error; deleteError != nil {
				return fmt.Errorf("delete RSVP calendar %s: %w", mapping.CalendarID, deleteError)
			}
		}
		sortedIDs := append([]string(nil), providerCalendarIDs...)
		sort.Strings(sortedIDs)
		for _, providerCalendarID := range sortedIDs {
			if existingMapping, found := existingByProviderID[providerCalendarID]; found {
				mappings = append(mappings, existingMapping)
				continue
			}
			providerCalendar := availableByID[providerCalendarID]
			displayOrder, orderError := models.NextCalendarDisplayOrder(transaction, organizer.ID)
			if orderError != nil {
				return orderError
			}
			calendar, calendarError := models.NewCalendar(organizer.ID, providerCalendar.Name, "G", providerCalendar.ColorToken, displayOrder)
			if calendarError != nil {
				return calendarError
			}
			if createError := transaction.Create(calendar).Error; createError != nil {
				return fmt.Errorf("create RSVP calendar for source %s: %w", providerCalendarID, createError)
			}
			mapping, mappingError := models.NewSourceCalendarMapping(connectionID, calendar.ID, providerCalendarID)
			if mappingError != nil {
				return mappingError
			}
			if createError := transaction.Create(mapping).Error; createError != nil {
				return fmt.Errorf("create source calendar mapping: %w", createError)
			}
			mappings = append(mappings, *mapping)
		}
		return normalizeCalendarOrder(transaction, organizer.ID)
	})
	return mappings, transactionError
}

// DeleteConnection deletes one connection, its credentials, and its empty mapped calendars.
func (service *CalendarConnectionService) DeleteConnection(ctx context.Context, organizerID string, connectionID string) error {
	return service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var connection models.CalendarConnection
		if findError := transaction.First(&connection, "id = ?", connectionID).Error; findError != nil {
			return findError
		}
		if connection.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		var mappings []models.SourceCalendarMapping
		if findError := transaction.Where("connection_id = ?", connectionID).Find(&mappings).Error; findError != nil {
			return findError
		}
		for _, mapping := range mappings {
			if deleteError := transaction.Unscoped().Delete(&mapping).Error; deleteError != nil {
				return fmt.Errorf("delete source calendar mapping %s: %w", mapping.ID, deleteError)
			}
			if deleteError := transaction.Unscoped().Delete(&models.Calendar{}, "id = ?", mapping.CalendarID).Error; deleteError != nil {
				return fmt.Errorf("delete source calendar %s: %w", mapping.CalendarID, deleteError)
			}
		}
		if deleteError := transaction.Unscoped().Delete(&connection).Error; deleteError != nil {
			return fmt.Errorf("delete calendar connection %s: %w", connectionID, deleteError)
		}
		if deleteError := transaction.Unscoped().Delete(&models.IdempotencyRecord{}, "resource_type = ? AND resource_id = ?", "calendar_connection", connectionID).Error; deleteError != nil {
			return fmt.Errorf("delete calendar connection idempotency record: %w", deleteError)
		}
		return normalizeCalendarOrder(transaction, organizerID)
	})
}

func (service *CalendarConnectionService) readIdempotentConnection(ctx context.Context, organizerID string, keyHash []byte, requestHash []byte) (*models.CalendarConnection, bool, error) {
	return service.readIdempotentConnectionWithDatabase(service.database.WithContext(ctx), organizerID, keyHash, requestHash)
}

func (service *CalendarConnectionService) readIdempotentConnectionWithDatabase(database *gorm.DB, organizerID string, keyHash []byte, requestHash []byte) (*models.CalendarConnection, bool, error) {
	var record models.IdempotencyRecord
	findError := database.First(&record, "organizer_id = ? AND operation = ? AND key_hash = ?", organizerID, createCalendarConnectionOperation, keyHash).Error
	if errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if findError != nil {
		return nil, false, findError
	}
	if !record.ExpiresAt.After(service.now().UTC()) {
		if deleteError := database.Unscoped().Delete(&record).Error; deleteError != nil {
			return nil, false, deleteError
		}
		return nil, false, nil
	}
	if subtle.ConstantTimeCompare(record.RequestHash, requestHash) != 1 {
		return nil, false, ErrIdempotencyConflict
	}
	var connection models.CalendarConnection
	if findError := database.First(&connection, "id = ?", record.ResourceID).Error; findError != nil {
		return nil, false, findError
	}
	return &connection, true, nil
}
