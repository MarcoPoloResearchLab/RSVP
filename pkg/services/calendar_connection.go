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
	"unicode"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	calendarAuthorizationLifetime     = 10 * time.Minute
	calendarCredentialRefreshSkew     = time.Minute
	idempotencyLifetime               = 24 * time.Hour
	createCalendarConnectionOperation = "create_calendar_connection"
	sourceCalendarLocalUseErrorCode   = "source_calendar_has_local_use"
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
	if findError := service.database.WithContext(ctx).Preload("Mappings.Calendar").First(&connection, "id = ?", connectionID).Error; findError != nil {
		return nil, findError
	}
	if connection.OrganizerID != organizerID {
		return nil, ErrResourceForbidden
	}
	return &connection, nil
}

// ReconcileSourceCalendars applies one complete or incremental CalendarList transition.
func (service *CalendarConnectionService) ReconcileSourceCalendars(ctx context.Context, organizerID string, connectionID string) ([]models.SourceCalendarMapping, error) {
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

	protectedMappingID := ""
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var lockedConnection models.CalendarConnection
		if findError := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedConnection, "id = ?", connectionID).Error; findError != nil {
			return findError
		}
		if lockedConnection.OrganizerID != organizerID {
			return ErrResourceForbidden
		}
		var existing []models.SourceCalendarMapping
		if findError := transaction.Preload("Calendar").Where("connection_id = ?", connectionID).Find(&existing).Error; findError != nil {
			return findError
		}
		existingBySource := make(map[string]models.SourceCalendarMapping, len(existing))
		existingByProviderID := make(map[string][]models.SourceCalendarMapping, len(existing))
		for _, mapping := range existing {
			existingBySource[sourceCalendarKey(mapping.ProviderCalendarID, mapping.SemanticGroup)] = mapping
			existingByProviderID[mapping.ProviderCalendarID] = append(existingByProviderID[mapping.ProviderCalendarID], mapping)
		}
		if lockedConnection.CalendarImportCutoverAt == nil {
			if !completeReconciliation {
				return errors.New("calendar import cutover requires a complete CalendarList reconciliation")
			}
			protectedCalendarIDs := make([]string, 0, len(existing))
			for _, mapping := range existing {
				protectedCalendarIDs = append(protectedCalendarIDs, mapping.CalendarID)
			}
			if cutoverError := deletePriorOrganizerCalendars(transaction, organizerID, protectedCalendarIDs); cutoverError != nil {
				return cutoverError
			}
		}
		providerCalendars := append([]ProviderCalendar(nil), batch.Calendars...)
		sort.Slice(providerCalendars, func(left int, right int) bool {
			return providerCalendars[left].ID < providerCalendars[right].ID
		})
		seenSources := make(map[string]struct{}, len(providerCalendars)+1)
		for _, providerCalendar := range providerCalendars {
			if providerCalendar.ID == "" {
				return errors.New("provider calendar ID is required")
			}
			if providerCalendar.Deleted || !providerCalendar.Readable {
				for _, existingMapping := range existingByProviderID[providerCalendar.ID] {
					if deleteError := deleteSourceCalendarMapping(transaction, existingMapping); deleteError != nil {
						if errors.Is(deleteError, ErrCalendarConnectionHasLocalUse) {
							protectedMappingID = existingMapping.ID
						}
						return deleteError
					}
					delete(existingBySource, sourceCalendarKey(existingMapping.ProviderCalendarID, existingMapping.SemanticGroup))
				}
				continue
			}
			for _, providerGroup := range providerCalendar.Groups {
				group, groupError := sourceCalendarGroup(providerGroup.Key)
				if groupError != nil || strings.TrimSpace(providerGroup.Name) == "" || strings.TrimSpace(providerGroup.ColorToken) == "" {
					return errors.Join(errors.New("semantic calendar group is invalid"), groupError)
				}
				sourceKey := sourceCalendarKey(providerCalendar.ID, group)
				if _, duplicate := seenSources[sourceKey]; duplicate {
					return errors.New("provider calendar batch contains a duplicate group")
				}
				seenSources[sourceKey] = struct{}{}
				if existingMapping, found := existingBySource[sourceKey]; found {
					expectedSymbol := calendarSymbol(providerGroup.Name)
					if existingMapping.Calendar.Name != providerGroup.Name || (lockedConnection.CalendarImportCutoverAt == nil && existingMapping.Calendar.Symbol != expectedSymbol) {
						existingMapping.Calendar.Name = providerGroup.Name
						if lockedConnection.CalendarImportCutoverAt == nil {
							existingMapping.Calendar.Symbol = expectedSymbol
						}
						if updateError := transaction.Save(&existingMapping.Calendar).Error; updateError != nil {
							return fmt.Errorf("update RSVP calendar %s from source %s: %w", existingMapping.CalendarID, providerCalendar.ID, updateError)
						}
					}
					continue
				}
				displayOrder, orderError := models.NextCalendarDisplayOrder(transaction, organizerID)
				if orderError != nil {
					return orderError
				}
				calendar, calendarError := models.NewCalendar(organizerID, providerGroup.Name, calendarSymbol(providerGroup.Name), providerGroup.ColorToken, displayOrder)
				if calendarError != nil {
					return calendarError
				}
				calendar.Visible = providerGroup.Visible
				if createError := transaction.Create(calendar).Error; createError != nil {
					return fmt.Errorf("create RSVP calendar for source %s: %w", providerCalendar.ID, createError)
				}
				if visibilityError := transaction.Model(calendar).Update("visible", providerGroup.Visible).Error; visibilityError != nil {
					return fmt.Errorf("set initial RSVP calendar visibility for source %s: %w", providerCalendar.ID, visibilityError)
				}
				mapping, mappingError := models.NewSourceCalendarMapping(connectionID, calendar.ID, providerCalendar.ID, group)
				if mappingError != nil {
					return mappingError
				}
				if createError := transaction.Create(mapping).Error; createError != nil {
					return fmt.Errorf("create source calendar mapping: %w", createError)
				}
				existingBySource[sourceKey] = *mapping
			}
		}
		if completeReconciliation {
			for sourceKey, mapping := range existingBySource {
				if _, found := seenSources[sourceKey]; found {
					continue
				}
				if deleteError := deleteSourceCalendarMapping(transaction, mapping); deleteError != nil {
					if errors.Is(deleteError, ErrCalendarConnectionHasLocalUse) {
						protectedMappingID = mapping.ID
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
		if protectedMappingID != "" {
			transactionError = errors.Join(transactionError, service.recordSourceReconciliationFailure(ctx, protectedMappingID))
		}
		return nil, transactionError
	}
	var mappings []models.SourceCalendarMapping
	if findError := service.database.WithContext(ctx).Where("connection_id = ?", connectionID).Order("provider_calendar_id ASC").Find(&mappings).Error; findError != nil {
		return nil, fmt.Errorf("list reconciled source calendar mappings: %w", findError)
	}
	return mappings, nil
}

func (service *CalendarConnectionService) recordSourceReconciliationFailure(ctx context.Context, mappingID string) error {
	startedAt := service.now().UTC()
	synchronization, synchronizationError := models.NewCalendarSync(mappingID, startedAt)
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

func sourceCalendarGroup(providerGroup ProviderCalendarGroupKey) (models.SourceCalendarGroup, error) {
	switch providerGroup {
	case ProviderCalendarGroupCalendar:
		return models.SourceCalendarGroupCalendar, nil
	case ProviderCalendarGroupBirthdays:
		return models.SourceCalendarGroupBirthdays, nil
	default:
		return "", errors.New("semantic calendar group is unknown")
	}
}

func sourceCalendarKey(providerCalendarID string, providerGroup models.SourceCalendarGroup) string {
	return providerCalendarID + "\x00" + string(providerGroup)
}

func calendarSymbol(calendarName string) string {
	for _, character := range strings.TrimSpace(calendarName) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			return strings.ToUpper(string(character))
		}
	}
	return "•"
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
	Mappings    []models.SourceCalendarMapping
}

// ReconcileAllSourceCalendars applies CalendarList changes for every connected organizer.
func (service *CalendarConnectionService) ReconcileAllSourceCalendars(ctx context.Context) ([]ReconciledCalendarConnection, error) {
	var connections []models.CalendarConnection
	if findError := service.database.WithContext(ctx).Where("status = ?", models.CalendarConnectionConnected).Order("id ASC").Find(&connections).Error; findError != nil {
		return nil, fmt.Errorf("list calendar connections: %w", findError)
	}
	reconciledConnections := make([]ReconciledCalendarConnection, 0, len(connections))
	var reconciliationErrors []error
	for _, connection := range connections {
		mappings, reconciliationError := service.ReconcileSourceCalendars(ctx, connection.OrganizerID, connection.ID)
		if reconciliationError != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("reconcile calendar connection %s: %w", connection.ID, reconciliationError))
			continue
		}
		reconciledConnections = append(reconciledConnections, ReconciledCalendarConnection{OrganizerID: connection.OrganizerID, Mappings: mappings})
	}
	return reconciledConnections, errors.Join(reconciliationErrors...)
}

func deleteSourceCalendarMapping(database *gorm.DB, mapping models.SourceCalendarMapping) error {
	if useError := requireSourceCalendarUnused(database, mapping); useError != nil {
		return useError
	}
	if deleteError := database.Unscoped().Delete(&mapping).Error; deleteError != nil {
		return fmt.Errorf("delete source calendar mapping %s: %w", mapping.ID, deleteError)
	}
	if deleteError := database.Unscoped().Delete(&models.Calendar{}, "id = ?", mapping.CalendarID).Error; deleteError != nil {
		return fmt.Errorf("delete RSVP calendar %s: %w", mapping.CalendarID, deleteError)
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
		var mappings []models.SourceCalendarMapping
		if findError := transaction.Where("connection_id = ?", connectionID).Find(&mappings).Error; findError != nil {
			return findError
		}
		for _, mapping := range mappings {
			if useError := requireSourceCalendarUnused(transaction, mapping); useError != nil {
				return useError
			}
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

func requireSourceMappingUnused(database *gorm.DB, mappingID string) error {
	var eventIDs []string
	if findError := database.Model(&models.ExternalEventLink{}).Where("mapping_id = ?", mappingID).Pluck("event_id", &eventIDs).Error; findError != nil {
		return findError
	}
	return requireSourceEventsUnused(database, eventIDs)
}

func requireSourceCalendarUnused(database *gorm.DB, mapping models.SourceCalendarMapping) error {
	var calendarLaneIDs []string
	if findError := database.Model(&models.Lane{}).Where("calendar_id = ?", mapping.CalendarID).Pluck("id", &calendarLaneIDs).Error; findError != nil {
		return findError
	}
	var calendarEventIDs []string
	if len(calendarLaneIDs) != 0 {
		if findError := database.Model(&models.Event{}).Where("lane_id IN ?", calendarLaneIDs).Pluck("id", &calendarEventIDs).Error; findError != nil {
			return findError
		}
	}

	type sourceEventIdentity struct {
		EventID string
		LaneID  string
	}
	var sourceEvents []sourceEventIdentity
	if findError := database.Model(&models.ExternalEventLink{}).
		Select("external_event_links.event_id, events.lane_id").
		Joins("JOIN events ON events.id = external_event_links.event_id AND events.deleted_at IS NULL").
		Where("external_event_links.mapping_id = ?", mapping.ID).
		Scan(&sourceEvents).Error; findError != nil {
		return findError
	}
	sourceEventIDs := make([]string, 0, len(sourceEvents))
	sourceLaneIDs := make([]string, 0, len(sourceEvents))
	for _, sourceEvent := range sourceEvents {
		sourceEventIDs = append(sourceEventIDs, sourceEvent.EventID)
		sourceLaneIDs = append(sourceLaneIDs, sourceEvent.LaneID)
	}
	if !sameIdentifierSet(calendarEventIDs, sourceEventIDs) || !sameIdentifierSet(calendarLaneIDs, sourceLaneIDs) {
		return ErrCalendarConnectionHasLocalUse
	}

	var calendarSeriesIDs []string
	if len(calendarLaneIDs) != 0 {
		if findError := database.Model(&models.EventSeries{}).Where("lane_id IN ?", calendarLaneIDs).Pluck("id", &calendarSeriesIDs).Error; findError != nil {
			return findError
		}
	}
	var sourceSeriesIDs []string
	if findError := database.Model(&models.ExternalEventSeriesLink{}).Where("mapping_id = ?", mapping.ID).Pluck("event_series_id", &sourceSeriesIDs).Error; findError != nil {
		return findError
	}
	if !sameIdentifierSet(calendarSeriesIDs, sourceSeriesIDs) {
		return ErrCalendarConnectionHasLocalUse
	}
	if useError := requireSourceEventsUnused(database, sourceEventIDs); useError != nil {
		return useError
	}

	type localUseCheck struct {
		model  any
		query  string
		values []any
	}
	checks := []localUseCheck{
		{model: &models.IngestionDraft{}, query: "calendar_id = ?", values: []any{mapping.CalendarID}},
	}
	if len(calendarLaneIDs) != 0 {
		checks = append(checks,
			localUseCheck{model: &models.AttentionPolicy{}, query: "lane_id IN ?", values: []any{calendarLaneIDs}},
			localUseCheck{model: &models.Probe{}, query: "lane_id IN ?", values: []any{calendarLaneIDs}},
			localUseCheck{model: &models.DerivedMarkerRule{}, query: "lane_id IN ?", values: []any{calendarLaneIDs}},
			localUseCheck{model: &models.DerivedMarker{}, query: "lane_id IN ?", values: []any{calendarLaneIDs}},
			localUseCheck{model: &models.DraftConfirmation{}, query: "lane_id IN ?", values: []any{calendarLaneIDs}},
		)
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

func sameIdentifierSet(first []string, second []string) bool {
	firstIdentifiers := make(map[string]struct{}, len(first))
	for _, identifier := range first {
		firstIdentifiers[identifier] = struct{}{}
	}
	secondIdentifiers := make(map[string]struct{}, len(second))
	for _, identifier := range second {
		secondIdentifiers[identifier] = struct{}{}
	}
	if len(firstIdentifiers) != len(secondIdentifiers) {
		return false
	}
	for identifier := range firstIdentifiers {
		if _, found := secondIdentifiers[identifier]; !found {
			return false
		}
	}
	return true
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
