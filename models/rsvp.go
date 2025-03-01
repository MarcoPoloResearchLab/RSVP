package models

import (
	"gorm.io/gorm"
)

// RSVP represents an invitation record with a base36 code, name, response, etc.
type RSVP struct {
	gorm.Model
	Name        string `gorm:"column:name"`
	Code        string `gorm:"column:code;uniqueIndex"`
	Response    string `gorm:"column:response"`
	ExtraGuests int    `gorm:"column:extra_guests;default:0"`
}

// FindByCode loads a single RSVP by its Code.
func (rsvp *RSVP) FindByCode(db *gorm.DB, code string) error {
	return db.Where("code = ?", code).First(rsvp).Error
}

// Create inserts a new RSVP into the database.
func (rsvp *RSVP) Create(db *gorm.DB) error {
	return db.Create(rsvp).Error
}

// Save updates an existing RSVP in the database.
func (rsvp *RSVP) Save(db *gorm.DB) error {
	return db.Save(rsvp).Error
}
