// Package models contains the database models for the RSVP system.
package models

import (
	"time"

	"gorm.io/gorm"
)

// Event represents an event created by a user.
type Event struct {
	BaseModel
	Title       string    `gorm:"size:255;not null"`
	Description string    `gorm:"type:text"`
	StartTime   time.Time `gorm:"not null"`
	EndTime     time.Time `gorm:"not null"`
	UserID      string    `gorm:"type:varchar(8);not null;index"`
	RSVPs       []RSVP    `gorm:"foreignKey:EventID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// DurationHours provides the event duration in hours for the UI edit form.
func (event *Event) DurationHours() int {
	eventDuration := event.EndTime.Sub(event.StartTime)
	return int(eventDuration.Hours())
}

// BeforeCreate is a GORM hook that generates a unique base62 ID for this Event.
func (event *Event) BeforeCreate(gormTransaction *gorm.DB) error {
	if event.ID == "" {
		uniqueID, uniqueIDError := EnsureUniqueID(gormTransaction, "events", GenerateBase62ID)
		if uniqueIDError != nil {
			return uniqueIDError
		}
		event.ID = uniqueID
	}
	return nil
}

// FindByID loads a single Event by its ID.
func (event *Event) FindByID(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Where("id = ?", eventIdentifier).First(event).Error
}

// FindByIDAndOwner loads a single Event by its ID, ensuring it belongs to the specified UserID.
// Populates the 'event' receiver if found and owned.
func (event *Event) FindByIDAndOwner(databaseConnection *gorm.DB, eventIdentifier string, ownerUserID string) error {
	return databaseConnection.Where("id = ? AND user_id = ?", eventIdentifier, ownerUserID).First(event).Error
}

// FindEventsByUserID retrieves all events belonging to a specific user.
// Optionally preloads associated RSVPs if preloadRSVPs is true.
func FindEventsByUserID(databaseConnection *gorm.DB, ownerUserID string, preloadRSVPs bool) ([]Event, error) {
	var userEvents []Event
	query := databaseConnection.Where("user_id = ?", ownerUserID).Order("start_time DESC") // Order descending by default

	if preloadRSVPs {
		query = query.Preload("RSVPs")
	}

	result := query.Find(&userEvents)
	return userEvents, result.Error
}

// Create inserts a new Event into the database.
func (event *Event) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(event).Error
}

// Save updates an existing Event in the database.
func (event *Event) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(event).Error
}

// LoadWithRSVPs loads an Event and its associated RSVPs.
// Note: This doesn't check ownership. Use FindByIDAndOwner first if needed.
func (event *Event) LoadWithRSVPs(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Preload("RSVPs").Where("id = ?", eventIdentifier).First(event).Error
}
