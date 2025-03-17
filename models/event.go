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

// DurationHours provides the event duration in hours for the UI edit form
func (event Event) DurationHours() int {
	duration := event.EndTime.Sub(event.StartTime)
	return int(duration.Hours())
}

// BeforeCreate hook to generate a unique base62 ID
func (event *Event) BeforeCreate(tx *gorm.DB) error {
	if event.ID == "" {
		id, err := EnsureUniqueID(tx, "events", GenerateBase62ID)
		if err != nil {
			return err
		}
		event.ID = id
	}
	return nil
}

func (event *Event) FindByID(db *gorm.DB, id string) error {
	return db.Where("id = ?", id).First(event).Error
}

func (event *Event) Create(db *gorm.DB) error {
	return db.Create(event).Error
}

func (event *Event) Save(db *gorm.DB) error {
	return db.Save(event).Error
}

func (event *Event) LoadWithRSVPs(db *gorm.DB, id string) error {
	return db.Preload("RSVPs").Where("id = ?", id).First(event).Error
}
