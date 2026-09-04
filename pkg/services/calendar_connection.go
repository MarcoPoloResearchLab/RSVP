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
	"sync"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	calendarAuthorizationLifetime      = 10 * time.Minute
	calendarCredentialRefreshSkew      = time.Minute
	idempotencyLifetime                = 24 * time.Hour
	createCalendarConnectionOperation  = "create_calendar_connection"
	calendarConnectionFallbackTimezone = "UTC"
	sourceCalendarLocalUseErrorCode    = "source_calendar_has_local_use"
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
	// ErrCalendarConnectionHasLocalUse indicates that local data depends on imported events.
	ErrCalendarConnectionHasLocalUse = errors.New("source calendar has local data")
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
	Timezone  string `json:"timezone"`
}

// CalendarConnectionService owns Google Calendar consent and source calendar reconciliation.
type CalendarConnectionService struct {
	database       *gorm.DB
	adapter        CalendarProviderAdapter
	cipher         *CredentialCipher
	now            func() time.Time
	random         io.Reader
	reconcileMutex sync.Mutex
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

// CreateConnection exchanges consent, stores encrypted credential data, and creates an import task.
func (service *CalendarConnectionService) CreateConnection(ctx context.Context, organizerID string, confirmation AuthorizationConfirmation, idempotencyKey string) (*models.CalendarConnection, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, ErrIdempotencyKeyRequired
	}
	timezone, timezoneError := models.NewTimezone(confirmation.Timezone)
	if timezoneError != nil {
		timezone, timezoneError = models.NewTimezone(calendarConnectionFallbackTimezone)
		if timezoneError != nil {
			return nil, false, timezoneError
		}
	}
	confirmation.Timezone = timezone.String()
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
		var organizer models.User
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&organizer, "id = ?", organizerID).Error; findError != nil {
			return findError
		}
		if confirmError := organizer.ConfirmTimezone(transaction, timezone); confirmError != nil {
			return confirmError
		}
		connection, connectionError := models.NewCalendarConnection(organizerID, nonce, ciphertext)
		if connectionError != nil {
			return connectionError
		}
		if createError := transaction.Create(connection).Error; createError != nil {
			return fmt.Errorf("create Google Calendar connection for organizer %s: %w", organizerID, createError)
		}
		importTask, taskError := models.NewCalendarConnectionImportTask(organizerID, connection.ID, service.now().UTC())
		if taskError != nil {
			return taskError
		}
		if createError := transaction.Create(importTask).Error; createError != nil {
			return fmt.Errorf("create calendar connection import task: %w", createError)
		}
		usedAt := service.now().UTC()
		if updateError := transaction.Model(&lockedRequest).Update("used_at", usedAt).Error; updateError != nil {
			return fmt.Errorf("use calendar authorization request %s: %w", lockedRequest.ID, updateError)
		}
		record, recordError := models.NewIdempotencyRecord(organizerID, createCalendarConnectionOperation, keyHash[:], requestHash[:], http.StatusAccepted, "calendar_connection", connection.ID, service.now().UTC().Add(idempotencyLifetime))
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
	if findError := service.database.WithContext(ctx).Preload("SyncStates.Mappings.Calendar").First(&connection, "id = ?", connectionID).Error; findError != nil {
		return nil, findError
	}
	if connection.OrganizerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &connection, nil
}

// ReconcileSourceCalendars applies one complete or incremental CalendarList transition.
func (service *CalendarConnectionService) ReconcileSourceCalendars(ctx context.Context, organizerID string, connectionID string) ([]models.ProviderCalendarSyncState, error) {
	service.reconcileMutex.Lock()
	defer service.reconcileMutex.Unlock()

	connection, connectionError := service.ReadConnection(ctx, organizerID, connectionID)
	if connectionError != nil {
		return nil, connectionError
	}
	credential, credentialError := currentCalendarCredential(ctx, service.database, service.adapter, service.cipher, connection, service.now())
	if credentialError != nil {
		return nil, credentialError
	}
	cursor := ""
	if connection.CalendarListSyncCursor != nil {
		cursor = *connection.CalendarListSyncCursor
	}
	batch, listError := service.adapter.ListCalendars(ctx, credential, cursor)
	completeReconciliation := cursor == ""
	if errors.Is(listError, ErrCalendarListSyncCursorRejected) && cursor != "" {
		batch, listError = service.adapter.ListCalendars(ctx, credential, "")
		completeReconciliation = true
	}
	if listError != nil {
		return nil, listError
	}
	if batch.NextSyncCursor == "" {
		return nil, errors.New("provider calendar batch has no sync cursor")
	}

	protectedSyncStateID := ""
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lockedConnection models.CalendarConnection
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedConnection, "id = ?", connectionID).Error; findError != nil {
			return findError
		}
		if lockedConnection.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		var existing []models.ProviderCalendarSyncState
		if findError := transaction.Preload("Mappings.Calendar").Where("connection_id = ?", connectionID).Find(&existing).Error; findError != nil {
			return findError
		}
		existingByProviderID := make(map[string]*models.ProviderCalendarSyncState, len(existing))
		var birthdaysCalendar *models.Calendar
		defaultCalendarID := ""
		for stateIndex := range existing {
			state := &existing[stateIndex]
			existingByProviderID[state.ProviderCalendarID] = state
			for mappingIndex := range state.Mappings {
				mapping := &state.Mappings[mappingIndex]
				if state.DefaultCalendar && mapping.SemanticGroup == models.SourceCalendarGroupCalendar {
					if defaultCalendarID != "" && defaultCalendarID != mapping.CalendarID {
						return errors.New("calendar connection has multiple default calendars")
					}
					defaultCalendarID = mapping.CalendarID
				}
				if mapping.SemanticGroup == models.SourceCalendarGroupBirthdays {
					if birthdaysCalendar != nil && birthdaysCalendar.ID != mapping.CalendarID {
						return errors.New("calendar connection has multiple Birthdays calendars")
					}
					calendar := mapping.Calendar
					birthdaysCalendar = &calendar
				}
			}
		}
		if lockedConnection.CalendarImportCutoverAt == nil {
			if !completeReconciliation {
				return errors.New("calendar import cutover requires a complete CalendarList reconciliation")
			}
			protectedCalendarIDs := make([]string, 0, len(existing)*2)
			for _, state := range existing {
				for _, mapping := range state.Mappings {
					protectedCalendarIDs = append(protectedCalendarIDs, mapping.CalendarID)
				}
			}
			if cutoverError := deletePriorOrganizerCalendars(transaction, organizerID, protectedCalendarIDs); cutoverError != nil {
				return cutoverError
			}
		}
		providerCalendars := append([]ProviderCalendar(nil), batch.Calendars...)
		sort.Slice(providerCalendars, func(left int, right int) bool {
			if providerCalendars[left].Default != providerCalendars[right].Default {
				return providerCalendars[left].Default
			}
			leftSemantic := providerCalendars[left].SemanticSourceGroup != nil
			rightSemantic := providerCalendars[right].SemanticSourceGroup != nil
			if leftSemantic != rightSemantic {
				return !leftSemantic
			}
			return providerCalendars[left].ID < providerCalendars[right].ID
		})
		seenProviderCalendars := make(map[string]struct{}, len(providerCalendars))
		for _, providerCalendar := range providerCalendars {
			if providerCalendar.ID == "" {
				return errors.New("provider calendar ID is required")
			}
			seenProviderCalendars[providerCalendar.ID] = struct{}{}
			existingState, found := existingByProviderID[providerCalendar.ID]
			if providerCalendar.Deleted || !providerCalendar.Readable {
				if found {
					if deleteError := deleteProviderCalendarSyncState(transaction, existingState); deleteError != nil {
						if errors.Is(deleteError, ErrCalendarConnectionHasLocalUse) {
							protectedSyncStateID = existingState.ID
						}
						return deleteError
					}
					delete(existingByProviderID, providerCalendar.ID)
				}
				continue
			}
			if !found {
				state, stateError := models.NewProviderCalendarSyncState(connectionID, providerCalendar.ID)
				if stateError != nil {
					return stateError
				}
				if createError := transaction.Create(state).Error; createError != nil {
					return fmt.Errorf("create provider calendar sync state: %w", createError)
				}
				existingState = state
				existingByProviderID[providerCalendar.ID] = state
			}
			if existingState.DefaultCalendar != providerCalendar.Default {
				if updateError := transaction.Model(existingState).Update("default_calendar", providerCalendar.Default).Error; updateError != nil {
					return fmt.Errorf("update provider default calendar identity: %w", updateError)
				}
				existingState.DefaultCalendar = providerCalendar.Default
			}
			semanticSource := providerCalendar.SemanticSourceGroup != nil
			if semanticSource && *providerCalendar.SemanticSourceGroup != SemanticCalendarGroupBirthdays {
				return errors.New("provider calendar has an unknown semantic source group")
			}
			if !semanticSource {
				if strings.TrimSpace(providerCalendar.Name) == "" || strings.TrimSpace(providerCalendar.ColorToken) == "" {
					return errors.New("provider calendar presentation is invalid")
				}
				calendarID, mappingError := ensureProviderCalendarMapping(transaction, organizerID, existingState, providerCalendar)
				if mappingError != nil {
					return mappingError
				}
				if providerCalendar.Default {
					if defaultCalendarID != "" && defaultCalendarID != calendarID {
						return errors.New("provider returned multiple default calendars")
					}
					defaultCalendarID = calendarID
				}
			} else {
				if defaultCalendarID == "" {
					return errors.New("semantic provider calendar requires a default RSVP calendar")
				}
				if mappingError := ensureSemanticMapping(transaction, existingState, defaultCalendarID, models.SourceCalendarGroupCalendar); mappingError != nil {
					return mappingError
				}
			}
			if birthdaysCalendar == nil {
				displayOrder, orderError := models.NextCalendarDisplayOrder(transaction, organizerID)
				if orderError != nil {
					return orderError
				}
				calendar, calendarError := models.NewCalendar(organizerID, "Birthdays", "birthdays", displayOrder)
				if calendarError != nil {
					return calendarError
				}
				visible, visibilityError := initialCalendarVisibility(transaction, organizerID, true)
				if visibilityError != nil {
					return visibilityError
				}
				calendar.Visible = visible
				if createError := transaction.Create(calendar).Error; createError != nil {
					return fmt.Errorf("create Birthdays calendar: %w", createError)
				}
				birthdaysCalendar = calendar
			}
			if mappingError := ensureSemanticMapping(transaction, existingState, birthdaysCalendar.ID, models.SourceCalendarGroupBirthdays); mappingError != nil {
				return mappingError
			}
		}
		if completeReconciliation {
			for providerCalendarID, state := range existingByProviderID {
				if _, found := seenProviderCalendars[providerCalendarID]; found {
					continue
				}
				if deleteError := deleteProviderCalendarSyncState(transaction, state); deleteError != nil {
					if errors.Is(deleteError, ErrCalendarConnectionHasLocalUse) {
						protectedSyncStateID = state.ID
					}
					return deleteError
				}
			}
		}
		connectionUpdates := map[string]any{"calendar_list_sync_cursor": batch.NextSyncCursor}
		if lockedConnection.CalendarImportCutoverAt == nil {
			connectionUpdates["calendar_import_cutover_at"] = service.now().UTC()
		}
		if updateError := transaction.Model(&lockedConnection).Updates(connectionUpdates).Error; updateError != nil {
			return fmt.Errorf("store CalendarList sync cursor: %w", updateError)
		}
		return normalizeCalendarOrder(transaction, organizerID)
	})
	if transactionError != nil {
		if protectedSyncStateID != "" {
			transactionError = errors.Join(transactionError, service.recordSourceReconciliationFailure(ctx, protectedSyncStateID))
		}
		return nil, transactionError
	}
	var syncStates []models.ProviderCalendarSyncState
	if findError := service.database.WithContext(ctx).Preload("Mappings.Calendar").Where("connection_id = ?", connectionID).Order("provider_calendar_id ASC").Find(&syncStates).Error; findError != nil {
		return nil, fmt.Errorf("list reconciled provider calendar states: %w", findError)
	}
	return syncStates, nil
}

func (service *CalendarConnectionService) recordSourceReconciliationFailure(ctx context.Context, syncStateID string) error {
	startedAt := service.now().UTC()
	synchronization, synchronizationError := models.NewCalendarSync(syncStateID, startedAt)
	if synchronizationError != nil {
		return synchronizationError
	}
	finishedAt := service.now().UTC()
	errorCode := sourceCalendarLocalUseErrorCode
	synchronization.State = models.CalendarSyncFailed
	synchronization.FinishedAt = &finishedAt
	synchronization.ErrorCode = &errorCode
	if createError := service.database.WithContext(ctx).Create(synchronization).Error; createError != nil {
		return fmt.Errorf("record source calendar reconciliation failure: %w", createError)
	}
	return nil
}

func ensureProviderCalendarMapping(database *gorm.DB, organizerID string, syncState *models.ProviderCalendarSyncState, providerCalendar ProviderCalendar) (string, error) {
	for mappingIndex := range syncState.Mappings {
		mapping := &syncState.Mappings[mappingIndex]
		if mapping.SemanticGroup != models.SourceCalendarGroupCalendar {
			continue
		}
		if mapping.Calendar.Name != providerCalendar.Name {
			if updateError := database.Model(&mapping.Calendar).Update("name", providerCalendar.Name).Error; updateError != nil {
				return "", fmt.Errorf("update RSVP calendar %s from source %s: %w", mapping.CalendarID, providerCalendar.ID, updateError)
			}
		}
		return mapping.CalendarID, nil
	}
	displayOrder, orderError := models.NextCalendarDisplayOrder(database, organizerID)
	if orderError != nil {
		return "", orderError
	}
	calendar, calendarError := models.NewCalendar(organizerID, providerCalendar.Name, providerCalendar.ColorToken, displayOrder)
	if calendarError != nil {
		return "", calendarError
	}
	visible, visibilityError := initialCalendarVisibility(database, organizerID, providerCalendar.Visible)
	if visibilityError != nil {
		return "", visibilityError
	}
	calendar.Visible = visible
	if createError := database.Create(calendar).Error; createError != nil {
		return "", fmt.Errorf("create RSVP calendar for source %s: %w", providerCalendar.ID, createError)
	}
	if visibilityError := database.Model(calendar).Update("visible", visible).Error; visibilityError != nil {
		return "", fmt.Errorf("set initial RSVP calendar visibility for source %s: %w", providerCalendar.ID, visibilityError)
	}
	if mappingError := ensureSemanticMapping(database, syncState, calendar.ID, models.SourceCalendarGroupCalendar); mappingError != nil {
		return "", mappingError
	}
	return calendar.ID, nil
}

func ensureSemanticMapping(database *gorm.DB, syncState *models.ProviderCalendarSyncState, calendarID string, group models.SourceCalendarGroup) error {
	for _, mapping := range syncState.Mappings {
		if mapping.SemanticGroup == group {
			if mapping.CalendarID != calendarID {
				return errors.New("semantic group mapping targets multiple RSVP calendars")
			}
			return nil
		}
	}
	mapping, mappingError := models.NewSourceCalendarMapping(syncState.ID, calendarID, group)
	if mappingError != nil {
		return mappingError
	}
	if createError := database.Create(mapping).Error; createError != nil {
		return fmt.Errorf("create semantic calendar mapping: %w", createError)
	}
	syncState.Mappings = append(syncState.Mappings, *mapping)
	return nil
}

func deletePriorOrganizerCalendars(database *gorm.DB, organizerID string, protectedCalendarIDs []string) error {
	query := database.Model(&models.Calendar{}).Where("organizer_id = ?", organizerID)
	if len(protectedCalendarIDs) != 0 {
		query = query.Where("id NOT IN ?", protectedCalendarIDs)
	}
	var calendarIDs []string
	if findError := query.Pluck("id", &calendarIDs).Error; findError != nil {
		return fmt.Errorf("list prior organizer calendars: %w", findError)
	}
	if len(calendarIDs) == 0 {
		return nil
	}
	if deleteError := database.Unscoped().Where("calendar_id IN ?", calendarIDs).Delete(&models.IngestionDraft{}).Error; deleteError != nil {
		return fmt.Errorf("delete prior calendar drafts: %w", deleteError)
	}
	if deleteError := database.Unscoped().Where("id IN ?", calendarIDs).Delete(&models.Calendar{}).Error; deleteError != nil {
		return fmt.Errorf("delete prior organizer calendars: %w", deleteError)
	}
	return nil
}

// ReconciledCalendarConnection identifies the mappings that are ready for event synchronization.
type ReconciledCalendarConnection struct {
	OrganizerID string
	SyncStates  []models.ProviderCalendarSyncState
}

// ReconcileAllSourceCalendars applies CalendarList changes without connections that an initial task owns.
func (service *CalendarConnectionService) ReconcileAllSourceCalendars(ctx context.Context, excludedConnectionIDs map[string]struct{}) ([]ReconciledCalendarConnection, error) {
	var connections []models.CalendarConnection
	if findError := service.database.WithContext(ctx).Where("status = ?", models.CalendarConnectionConnected).Order("id ASC").Find(&connections).Error; findError != nil {
		return nil, fmt.Errorf("list calendar connections: %w", findError)
	}
	reconciledConnections := make([]ReconciledCalendarConnection, 0, len(connections))
	var reconciliationErrors []error
	for _, connection := range connections {
		if _, excluded := excludedConnectionIDs[connection.ID]; excluded {
			continue
		}
		mappings, reconciliationError := service.ReconcileSourceCalendars(ctx, connection.OrganizerID, connection.ID)
		if reconciliationError != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("reconcile calendar connection %s: %w", connection.ID, reconciliationError))
			continue
		}
		reconciledConnections = append(reconciledConnections, ReconciledCalendarConnection{OrganizerID: connection.OrganizerID, SyncStates: mappings})
	}
	return reconciledConnections, errors.Join(reconciliationErrors...)
}

func deleteProviderCalendarSyncState(database *gorm.DB, syncState *models.ProviderCalendarSyncState) error {
	if useError := requireProviderCalendarUnused(database, syncState.ID); useError != nil {
		return useError
	}
	var links []models.ExternalEventLink
	if findError := database.Where("sync_state_id = ?", syncState.ID).Find(&links).Error; findError != nil {
		return findError
	}
	for _, link := range links {
		if deleteError := deleteExternalEvent(database, syncState.ID, link.ProviderEventID); deleteError != nil {
			return deleteError
		}
	}
	calendarIDs := make([]string, 0, len(syncState.Mappings))
	for _, mapping := range syncState.Mappings {
		calendarIDs = append(calendarIDs, mapping.CalendarID)
	}
	if deleteError := database.Unscoped().Delete(syncState).Error; deleteError != nil {
		return fmt.Errorf("delete provider calendar state %s: %w", syncState.ID, deleteError)
	}
	for _, calendarID := range calendarIDs {
		var mappingCount int64
		if countError := database.Model(&models.SourceCalendarMapping{}).Where("calendar_id = ?", calendarID).Count(&mappingCount).Error; countError != nil {
			return countError
		}
		if mappingCount != 0 {
			continue
		}
		var laneCount int64
		if countError := database.Model(&models.Lane{}).Where("calendar_id = ?", calendarID).Count(&laneCount).Error; countError != nil {
			return countError
		}
		if laneCount != 0 {
			return ErrCalendarConnectionHasLocalUse
		}
		if deleteError := database.Unscoped().Delete(&models.Calendar{}, "id = ?", calendarID).Error; deleteError != nil {
			return fmt.Errorf("delete RSVP calendar %s: %w", calendarID, deleteError)
		}
	}
	return nil
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
		if deleteError := transaction.Unscoped().Delete(&models.Task{}, "organizer_id = ? AND resource_type = ? AND resource_id = ?", organizerID, models.TaskResourceCalendarConnection, connectionID).Error; deleteError != nil {
			return fmt.Errorf("delete calendar connection tasks: %w", deleteError)
		}
		var syncStates []models.ProviderCalendarSyncState
		if findError := transaction.Preload("Mappings").Where("connection_id = ?", connectionID).Find(&syncStates).Error; findError != nil {
			return findError
		}
		for stateIndex := range syncStates {
			if deleteError := deleteProviderCalendarSyncState(transaction, &syncStates[stateIndex]); deleteError != nil {
				return deleteError
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

func currentCalendarCredential(ctx context.Context, database *gorm.DB, adapter CalendarProviderAdapter, credentialCipher *CredentialCipher, connection *models.CalendarConnection, now time.Time) (CalendarProviderCredential, error) {
	credential, credentialError := credentialCipher.decrypt(connection.CredentialNonce, connection.CredentialCiphertext)
	if credentialError != nil {
		return CalendarProviderCredential{}, credentialError
	}
	if credential.ExpiresAt.After(now.UTC().Add(calendarCredentialRefreshSkew)) {
		return credential, nil
	}
	refreshed, refreshError := adapter.RefreshCredential(ctx, credential)
	if refreshError != nil {
		return CalendarProviderCredential{}, refreshError
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || !refreshed.ExpiresAt.After(now.UTC()) {
		return CalendarProviderCredential{}, errors.New("refreshed calendar credential is invalid")
	}
	nonce, ciphertext, encryptionError := credentialCipher.encrypt(refreshed)
	if encryptionError != nil {
		return CalendarProviderCredential{}, encryptionError
	}
	connection.CredentialNonce = nonce
	connection.CredentialCiphertext = ciphertext
	if updateError := database.WithContext(ctx).Model(connection).Updates(map[string]any{"credential_nonce": nonce, "credential_ciphertext": ciphertext}).Error; updateError != nil {
		return CalendarProviderCredential{}, fmt.Errorf("store refreshed calendar credential: %w", updateError)
	}
	return refreshed, nil
}

func requireProviderCalendarUnused(database *gorm.DB, syncStateID string) error {
	var eventIDs []string
	if findError := database.Model(&models.ExternalEventLink{}).Where("sync_state_id = ?", syncStateID).Pluck("event_id", &eventIDs).Error; findError != nil {
		return findError
	}
	return requireSourceEventsUnused(database, eventIDs)
}

func requireSourceEventUnused(database *gorm.DB, eventID string) error {
	return requireSourceEventsUnused(database, []string{eventID})
}

func requireSourceEventsUnused(database *gorm.DB, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	checks := []struct {
		model  any
		query  string
		values []any
	}{
		{model: &models.Event{}, query: "id IN ? AND venue_id IS NOT NULL", values: []any{eventIDs}},
		{model: &models.RSVP{}, query: "event_id IN ?", values: []any{eventIDs}},
		{model: &models.Event{}, query: "anchor_event_id IN ? AND relation_type = ?", values: []any{eventIDs, models.EventRelationDependent}},
		{model: &models.DerivedMarkerRule{}, query: "anchor_type = ? AND anchor_id IN ?", values: []any{models.DerivedAnchorEvent, eventIDs}},
		{model: &models.IngestionDraft{}, query: "anchor_event_id IN ? AND status IN ?", values: []any{eventIDs, []models.IngestionDraftStatus{models.IngestionDraftIncomplete, models.IngestionDraftReady}}},
	}
	for _, check := range checks {
		var count int64
		if countError := database.Model(check.model).Where(check.query, check.values...).Count(&count).Error; countError != nil {
			return countError
		}
		if count != 0 {
			return ErrCalendarConnectionHasLocalUse
		}
	}
	return nil
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
