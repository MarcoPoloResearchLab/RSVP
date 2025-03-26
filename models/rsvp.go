// Package models contains the database models for the RSVP system.
package models

import (
	"gorm.io/gorm"
)

// RSVP represents an invitation record with a base36 ID, name, response, etc.
type RSVP struct {
	BaseModel
	Name        string `gorm:"column:name"`
	Response    string `gorm:"column:response"`
	ExtraGuests int    `gorm:"column:extra_guests;default:0"`
	EventID     string `gorm:"type:varchar(8);not null;index"`
}

// BeforeCreate is a GORM hook that generates a unique base36 ID for this RSVP.
func (rsvpRecord *RSVP) BeforeCreate(gormTransaction *gorm.DB) error {
	if rsvpRecord.ID == "" {
		uniqueID, uniqueIDError := EnsureUniqueID(gormTransaction, "rsvps", GenerateBase36ID)
		if uniqueIDError != nil {
			return uniqueIDError
		}
		rsvpRecord.ID = uniqueID
	}
	return nil
}

// FindByCode loads a single RSVP by its ID (used as the code).
func (rsvpRecord *RSVP) FindByCode(databaseConnection *gorm.DB, rsvpCode string) error {
	return databaseConnection.Where("id = ?", rsvpCode).First(rsvpRecord).Error
}

// FindByIDAndEventID loads a single RSVP by its ID, ensuring it belongs to the specified EventID.
// Populates the 'rsvpRecord' receiver if found and associated.
func (rsvpRecord *RSVP) FindByIDAndEventID(databaseConnection *gorm.DB, rsvpIdentifier string, parentEventID string) error {
	return databaseConnection.Where("id = ? AND event_id = ?", rsvpIdentifier, parentEventID).First(rsvpRecord).Error
}

// FindRSVPsByEventID retrieves all RSVP records associated with a specific event ID.
// Results are ordered by Name ascending.
func FindRSVPsByEventID(databaseConnection *gorm.DB, parentEventID string) ([]RSVP, error) {
	var eventRSVPs []RSVP
	result := databaseConnection.Where("event_id = ?", parentEventID).Order("name ASC").Find(&eventRSVPs)
	return eventRSVPs, result.Error
}

// Create inserts a new RSVP into the database.
func (rsvpRecord *RSVP) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(rsvpRecord).Error
}

// Save updates an existing RSVP in the database.
func (rsvpRecord *RSVP) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(rsvpRecord).Error
}
