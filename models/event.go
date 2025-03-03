package models

import (
	"gorm.io/gorm"
	"time"
)

// Event represents an event created by a user.
type Event struct {
	gorm.Model
	Title       string    `gorm:"size:255;not null"`
	Description string    `gorm:"type:text"`
	StartTime   time.Time `gorm:"not null"`
	EndTime     time.Time `gorm:"not null"`
	UserID      uint      `gorm:"not null;index"`
	RSVPs       []RSVP    `gorm:"foreignKey:EventID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (event *Event) FindByID(db *gorm.DB, id uint) error {
	return db.First(event, id).Error
}

func (event *Event) Create(db *gorm.DB) error {
	return db.Create(event).Error
}

func (event *Event) Save(db *gorm.DB) error {
	return db.Save(event).Error
}

func (event *Event) LoadWithRSVPs(db *gorm.DB, id uint) error {
	return db.Preload("RSVPs").First(event, id).Error
}
