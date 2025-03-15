package models

import (
	"errors"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email   string  `gorm:"uniqueIndex;size:255;not null"`
	Name    string  `gorm:"size:255"`
	Picture string  `gorm:"size:512"`
	Events  []Event `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// FindByEmail searches for a user by their email.
func (user *User) FindByEmail(db *gorm.DB, email string) error {
	return db.Where("email = ?", email).First(user).Error
}

// Create inserts a new user into the database.
func (user *User) Create(db *gorm.DB) error {
	return db.Create(user).Error
}

// Save updates an existing user in the database.
func (user *User) Save(db *gorm.DB) error {
	return db.Save(user).Error
}

// LoadWithEvents preloads the user's events.
func (user *User) LoadWithEvents(db *gorm.DB, userID uint) error {
	return db.Preload("Events").First(user, userID).Error
}

// UpsertUser creates or updates a user record based on the email.
func UpsertUser(
	db *gorm.DB,
	email string,
	name string,
	picture string,
) (*User, error) {
	var user User

	result := db.First(&user, "email = ?", email)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// New user, create
			user = User{
				Email:   email,
				Name:    name,
				Picture: picture,
			}
			if err := db.Create(&user).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, result.Error
		}
	} else {
		// Existing user, update fields if changed
		updated := false
		if user.Name != name {
			user.Name = name
			updated = true
		}
		if user.Picture != picture {
			user.Picture = picture
			updated = true
		}
		if updated {
			if err := db.Save(&user).Error; err != nil {
				return nil, err
			}
		}
	}

	return &user, nil
}
