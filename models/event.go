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
func (event Event) DurationHours() int {
	eventDuration := event.EndTime.Sub(event.StartTime)
	return int(eventDuration.Hours())
}

// BeforeCreate hook to generate a unique base62 ID.
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

func (event *Event) FindByID(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Where("id = ?", eventIdentifier).First(event).Error
}

func (event *Event) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(event).Error
}

func (event *Event) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(event).Error
}

func (event *Event) LoadWithRSVPs(databaseConnection *gorm.DB, eventIdentifier string) error {
	return databaseConnection.Preload("RSVPs").Where("id = ?", eventIdentifier).First(event).Error
}
