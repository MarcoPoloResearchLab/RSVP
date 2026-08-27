package models

import (
	"errors"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

var ErrIdempotencyRecordInvalid = errors.New("idempotency record is invalid")

// IdempotencyRecord connects one request key to one created resource.
type IdempotencyRecord struct {
	BaseModel
	OrganizerID    string    `gorm:"type:varchar(8);not null;uniqueIndex:idempotency_operation_key"`
	Operation      string    `gorm:"not null;uniqueIndex:idempotency_operation_key"`
	KeyHash        []byte    `gorm:"not null;uniqueIndex:idempotency_operation_key"`
	RequestHash    []byte    `gorm:"not null"`
	ResponseStatus int       `gorm:"not null"`
	ResourceType   string    `gorm:"not null"`
	ResourceID     string    `gorm:"type:varchar(8);not null"`
	ExpiresAt      time.Time `gorm:"not null"`
	Organizer      User      `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewIdempotencyRecord constructs one valid request result record.
func NewIdempotencyRecord(organizerID string, operation string, keyHash []byte, requestHash []byte, responseStatus int, resourceType string, resourceID string, expiresAt time.Time) (*IdempotencyRecord, error) {
	record := &IdempotencyRecord{OrganizerID: organizerID, Operation: operation, KeyHash: append([]byte(nil), keyHash...), RequestHash: append([]byte(nil), requestHash...), ResponseStatus: responseStatus, ResourceType: resourceType, ResourceID: resourceID, ExpiresAt: expiresAt.UTC()}
	if validationError := record.Validate(); validationError != nil {
		return nil, validationError
	}
	return record, nil
}

func (record *IdempotencyRecord) Validate() error {
	if record.OrganizerID == "" || record.Operation == "" || len(record.KeyHash) != 32 || len(record.RequestHash) != 32 || record.ResponseStatus < 200 || record.ResourceType == "" || record.ResourceID == "" || record.ExpiresAt.IsZero() {
		return ErrIdempotencyRecordInvalid
	}
	return nil
}

func (record *IdempotencyRecord) BeforeCreate(database *gorm.DB) error {
	if validationError := record.Validate(); validationError != nil {
		return validationError
	}
	return record.BaseModel.GenerateID(database, record)
}
func (record *IdempotencyRecord) BeforeUpdate(*gorm.DB) error { return record.Validate() }
func (record *IdempotencyRecord) GetTableName() string        { return config.TableIdempotencyRecords }
func (record *IdempotencyRecord) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}
