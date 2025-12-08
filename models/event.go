package models

import (
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// Event represents a scheduled event in the system.
type Event struct {
	BaseModel
	Title       string `gorm:"not null"`
	Description string
	StartTime   time.Time `gorm:"not null"`
	EndTime     time.Time `gorm:"not null"`
	UserID      string    `gorm:"not null;index"`
	VenueID     *string   `gorm:"type:varchar(8);index"`
	RSVPs       []RSVP    `gorm:"foreignKey:EventID"`
	User        User      `gorm:"foreignKey:UserID"`
	Venue       *Venue    `gorm:"foreignKey:VenueID;references:id"`
}

// DurationHours returns the event duration in hours.
func (eventInstance *Event) DurationHours() int {
	eventDuration := eventInstance.EndTime.Sub(eventInstance.StartTime)
	return int(eventDuration.Hours())
}

// GetTableName returns the database table name for the Event model.
func (eventInstance *Event) GetTableName() string {
	return config.TableEvents
}

// GetIDGeneratorFunc returns the unique ID generation function for the Event model.
func (eventInstance *Event) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// BeforeCreate is a GORM hook to ensure the event has a unique ID before creation.
func (eventInstance *Event) BeforeCreate(databaseTransaction *gorm.DB) error {
	generateIDError := eventInstance.BaseModel.GenerateID(databaseTransaction, eventInstance)
	if generateIDError != nil {
		return generateIDError
	}
	return nil
}

// FindByID retrieves an Event record by its identifier.
func (eventInstance *Event) FindByID(databaseConnection *gorm.DB, eventIdentifier string) error {
	queryError := databaseConnection.Where("id = ?", eventIdentifier).First(eventInstance).Error
	return queryError
}

// FindByIDAndOwner retrieves an Event record by its identifier ensuring it belongs to the specified owner.
// It also preloads the associated Venue.
func (eventInstance *Event) FindByIDAndOwner(databaseConnection *gorm.DB, eventIdentifier string, ownerUserID string) error {
	queryError := databaseConnection.Preload("Venue").Where("id = ? AND user_id = ?", eventIdentifier, ownerUserID).First(eventInstance).Error
	return queryError
}

// FindEventsByUserID retrieves all events for a given user identifier.
func FindEventsByUserID(databaseConnection *gorm.DB, ownerUserID string, preloadRSVPs bool, preloadVenues bool) ([]Event, error) {
	var userEvents []Event
	queryBuilder := databaseConnection.Where("user_id = ?", ownerUserID).Order("start_time DESC")
	if preloadRSVPs {
		queryBuilder = queryBuilder.Preload("RSVPs")
	}
	if preloadVenues {
		queryBuilder = queryBuilder.Preload("Venue")
	}
	queryError := queryBuilder.Find(&userEvents).Error
	return userEvents, queryError
}

// Create inserts the current Event record into the database.
func (eventInstance *Event) Create(databaseConnection *gorm.DB) error {
	if eventInstance.VenueID == nil {
		eventInstance.Venue = nil
	}
	creationError := databaseConnection.Create(eventInstance).Error
	return creationError
}

// Update updates the current Event record in the database.
func (eventInstance *Event) Update(databaseConnection *gorm.DB) error {
	if eventInstance.VenueID == nil {
		eventInstance.Venue = nil
	}
	updateError := databaseConnection.Save(eventInstance).Error
	return updateError
}

// LoadWithRSVPs retrieves an Event record along with its associated RSVPs.
func (eventInstance *Event) LoadWithRSVPs(databaseConnection *gorm.DB, eventIdentifier string) error {
	queryError := databaseConnection.Preload("RSVPs").Where("id = ?", eventIdentifier).First(eventInstance).Error
	return queryError
}

// LoadWithVenue retrieves an Event record along with its associated Venue.
func (eventInstance *Event) LoadWithVenue(databaseConnection *gorm.DB, eventIdentifier string) error {
	queryError := databaseConnection.Preload("Venue").Where("id = ?", eventIdentifier).First(eventInstance).Error
	return queryError
}

// FindVenueIDsAssociatedWithUserEvents retrieves distinct venue IDs associated with events for a given user.
func FindVenueIDsAssociatedWithUserEvents(databaseConnection *gorm.DB, ownerUserID string) ([]string, error) {
	var venueIdentifierList []string
	queryError := databaseConnection.Model(&Event{}).
		Where("user_id = ? AND venue_id IS NOT NULL", ownerUserID).
		Distinct("venue_id").
		Pluck("venue_id", &venueIdentifierList).Error
	return venueIdentifierList, queryError
}
