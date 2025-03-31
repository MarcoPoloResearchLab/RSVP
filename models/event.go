package models

import (
	"time"

	"github.com/temirov/RSVP/pkg/config" // Import config
	"gorm.io/gorm"
)

// Event represents an event created by a user, containing details like title,
// description, start/end times, owner (UserID), and associated RSVPs.
type Event struct {
	BaseModel
	// Title is the name of the event (required, max 255 chars).
	Title string `gorm:"size:255;not null"`
	// Description provides additional details about the event (optional).
	Description string `gorm:"type:text"`
	// StartTime is the date and time when the event begins (required).
	StartTime time.Time `gorm:"not null"`
	// EndTime is the date and time when the event ends (required).
	EndTime time.Time `gorm:"not null"`
	// UserID links the event to the User who created it (required). Indexed for performance.
	UserID string `gorm:"type:varchar(8);not null;index"`
	// RSVPs is a slice containing all RSVP records associated with this event.
	// GORM automatically handles the foreign key relationship (EventID on RSVP model).
	// Cascade constraints ensure RSVPs are updated/deleted when the event is.
	RSVPs []RSVP `gorm:"foreignKey:EventID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// DurationHours calculates and returns the event's duration in whole hours.
// This is primarily used for populating the edit form in the UI.
func (event *Event) DurationHours() int {
	eventDuration := event.EndTime.Sub(event.StartTime)
	return int(eventDuration.Hours())
}

// BeforeCreate is a GORM hook executed before a new Event record is inserted into the database.
// It ensures that the Event has a unique base62 ID generated if one is not already set.
func (event *Event) BeforeCreate(dbTransaction *gorm.DB) error {
	// Only generate an ID if the ID field is currently empty.
	if event.ID == "" {
		// Ensure a unique ID is generated for the 'events' table using the Base62 generator.
		uniqueID, uniqueIDError := EnsureUniqueID(dbTransaction, config.TableEvents, GenerateBase62ID)
		if uniqueIDError != nil {
			// Return the error if ID generation or uniqueness check fails.
			return uniqueIDError
		}
		// Assign the generated unique ID to the event record.
		event.ID = uniqueID
	}
	// Return nil to indicate success.
	return nil
}

// FindByID retrieves a single Event from the database based on its primary key (ID).
// It populates the receiver 'event' struct with the found data.
// Returns an error if the record is not found or if a database error occurs.
func (event *Event) FindByID(databaseConnection *gorm.DB, eventIdentifier string) error {
	// Query the database for an event with the matching ID and populate the event struct.
	return databaseConnection.Where("id = ?", eventIdentifier).First(event).Error
}

// FindByIDAndOwner retrieves a single Event by its ID, but only if it belongs to the specified ownerUserID.
// This is crucial for authorization checks, ensuring users can only access their own events.
// Populates the 'event' receiver if found and owned by the specified user.
// Returns gorm.ErrRecordNotFound if the event doesn't exist or doesn't belong to the user.
func (event *Event) FindByIDAndOwner(databaseConnection *gorm.DB, eventIdentifier string, ownerUserID string) error {
	// Query for an event matching both the ID and the UserID.
	return databaseConnection.Where("id = ? AND user_id = ?", eventIdentifier, ownerUserID).First(event).Error
}

// FindEventsByUserID retrieves all events associated with a specific user ID.
// Events are ordered by start time in descending order (most recent first).
// Optionally, it can preload the associated RSVP records for each event if preloadRSVPs is true.
func FindEventsByUserID(databaseConnection *gorm.DB, ownerUserID string, preloadRSVPs bool) ([]Event, error) {
	var userEvents []Event
	// Base query: select events for the given user, ordered by start time descending.
	query := databaseConnection.Where("user_id = ?", ownerUserID).Order("start_time DESC")

	// If requested, preload the associated RSVPs for each event.
	if preloadRSVPs {
		query = query.Preload("RSVPs")
	}

	// Execute the query and find all matching events.
	result := query.Find(&userEvents)
	// Return the slice of events and any error encountered during the query.
	return userEvents, result.Error
}

// Create inserts the current Event struct instance (the receiver 'event') as a new record into the database.
// It triggers the BeforeCreate hook to generate an ID if necessary.
// Returns an error if the database insertion fails.
func (event *Event) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(event).Error
}

// Save updates the existing Event record in the database corresponding to the receiver 'event' struct's ID.
// It updates all fields based on the current values in the struct.
// Returns an error if the database update fails.
func (event *Event) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(event).Error
}

// LoadWithRSVPs retrieves a single Event by its ID and preloads its associated RSVP records.
// This method does *not* perform ownership checks. Use FindByIDAndOwner first if authorization is needed.
// Populates the receiver 'event' struct with the event data and its RSVPs.
func (event *Event) LoadWithRSVPs(databaseConnection *gorm.DB, eventIdentifier string) error {
	// Query for the event by ID and preload the RSVPs relationship.
	return databaseConnection.Preload("RSVPs").Where("id = ?", eventIdentifier).First(event).Error
}
