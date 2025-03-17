package models

import (
	"gorm.io/gorm"
)

// RSVP represents an invitation record with a base36 ID, name, response, etc.
type RSVP struct {
	BaseModel    // ID field will be used as the RSVP code
	Name        string `gorm:"column:name"`
	Response    string `gorm:"column:response"`
	ExtraGuests int    `gorm:"column:extra_guests;default:0"`
	EventID     string `gorm:"type:varchar(8);not null;index"`
}

// BeforeCreate hook to generate a unique base36 ID
func (rsvp *RSVP) BeforeCreate(tx *gorm.DB) error {
	if rsvp.ID == "" {
		id, err := EnsureUniqueID(tx, "rsvps", GenerateBase36ID)
		if err != nil {
			return err
		}
		rsvp.ID = id
	}
	return nil
}

// FindByCode loads a single RSVP by its ID (which is used as the code).
func (rsvp *RSVP) FindByCode(db *gorm.DB, code string) error {
	return db.Where("id = ?", code).First(rsvp).Error
}

// Create inserts a new RSVP into the database.
func (rsvp *RSVP) Create(db *gorm.DB) error {
	return db.Create(rsvp).Error
}

// Save updates an existing RSVP in the database.
func (rsvp *RSVP) Save(db *gorm.DB) error {
	return db.Save(rsvp).Error
}
