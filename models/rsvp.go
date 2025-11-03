package models

import (
	"github.com/tyemirov/RSVP/pkg/config" // Import config
	"gorm.io/gorm"
)

// RSVP represents an invitation record for a specific event.
// Each RSVP has a unique base36 ID (used as the public access code),
// an optional invitee name, their response status, number of extra guests,
// and a link back to the parent Event.
type RSVP struct {
	BaseModel
	// Name is the name of the invitee (optional but recommended).
	Name string `gorm:"column:name"`
	// Response stores the invitee's response status (e.g., "Pending", "Yes,1", "No,0").
	// See config constants like RSVPResponseYesPlusOne, RSVPResponseNoCommaZero.
	Response string `gorm:"column:response"`
	// ExtraGuests indicates the number of additional guests the invitee is bringing (0-4).
	// Defaults to 0. Only relevant if Response starts with "Yes".
	ExtraGuests int `gorm:"column:extra_guests;default:0"`
	// EventID links the RSVP to the parent Event (required). Indexed for performance.
	EventID string `gorm:"type:varchar(8);not null;index"`
}

// BeforeCreate is a GORM hook executed before a new RSVP record is inserted.
// It ensures that the RSVP has a unique base36 ID generated if one is not already set.
// Base36 is used for shorter, more user-friendly codes compared to base62.
func (rsvpRecord *RSVP) BeforeCreate(dbTransaction *gorm.DB) error {
	// Only generate an ID if the ID field is currently empty.
	if rsvpRecord.ID == "" {
		// Ensure a unique ID is generated for the 'rsvps' table using the Base36 generator.
		uniqueID, uniqueIDError := EnsureUniqueID(dbTransaction, config.TableRSVPs, GenerateBase36ID)
		if uniqueIDError != nil {
			// Return error if ID generation or uniqueness check fails.
			return uniqueIDError
		}
		// Assign the generated unique ID.
		rsvpRecord.ID = uniqueID
	}
	// Return nil to indicate success.
	return nil
}

// FindByCode retrieves a single RSVP record from the database based on its ID (which serves as the public code).
// It populates the receiver 'rsvpRecord' struct with the found data.
// Returns an error (like gorm.ErrRecordNotFound) if the record is not found or if a database error occurs.
func (rsvpRecord *RSVP) FindByCode(databaseConnection *gorm.DB, rsvpCode string) error {
	// Query the database for an RSVP with the matching ID.
	return databaseConnection.Where("id = ?", rsvpCode).First(rsvpRecord).Error
}

// FindByIDAndEventID retrieves a single RSVP by its ID, ensuring it belongs to the specified parentEventID.
// This can be used for authorization checks within the context of a specific event.
// Populates the 'rsvpRecord' receiver if found and associated with the correct event.
// Returns gorm.ErrRecordNotFound if the RSVP doesn't exist or isn't linked to the given event.
func (rsvpRecord *RSVP) FindByIDAndEventID(databaseConnection *gorm.DB, rsvpIdentifier string, parentEventID string) error {
	// Query for an RSVP matching both the ID and the EventID.
	return databaseConnection.Where("id = ? AND event_id = ?", rsvpIdentifier, parentEventID).First(rsvpRecord).Error
}

// FindRSVPsByEventID retrieves all RSVP records associated with a specific event ID.
// Results are ordered alphabetically by the invitee's Name.
func FindRSVPsByEventID(databaseConnection *gorm.DB, parentEventID string) ([]RSVP, error) {
	var eventRSVPs []RSVP
	// Query for RSVPs matching the event ID, ordered by name ascending.
	result := databaseConnection.Where("event_id = ?", parentEventID).Order("name ASC").Find(&eventRSVPs)
	// Return the slice of RSVPs and any error from the query.
	return eventRSVPs, result.Error
}

// Create inserts the current RSVP struct instance (the receiver 'rsvpRecord') as a new record into the database.
// Triggers the BeforeCreate hook to generate an ID if necessary.
// Returns an error if the database insertion fails.
func (rsvpRecord *RSVP) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(rsvpRecord).Error
}

// Save updates the existing RSVP record in the database corresponding to the receiver 'rsvpRecord' struct's ID.
// Updates all fields based on the current values in the struct.
// Returns an error if the database update fails.
func (rsvpRecord *RSVP) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(rsvpRecord).Error
}
