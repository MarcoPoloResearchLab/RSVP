// Package models contains the database models (structs corresponding to tables)
// and related database interaction logic for the RSVP system.
package models

import (
	"errors"
	"fmt"
	"log" // Keep log for UpsertUser specific logging

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// User represents an authenticated user in the system, typically identified via Google OAuth.
// It stores basic profile information and links to the events they have created.
type User struct {
	BaseModel
	// Email is the user's unique email address (used as the primary identifier from OAuth).
	// Must be unique and is indexed for fast lookups.
	Email string `gorm:"uniqueIndex;size:255;not null"`
	// Name is the user's full name, as provided by the authentication provider.
	Name string `gorm:"size:255"`
	// Picture is the URL to the user's profile picture, provided by the authentication provider.
	Picture string `gorm:"size:512"`
	// Timezone is absent until the client confirms an IANA timezone.
	Timezone *string `gorm:"type:text"`
	// Calendars contain all temporal resources that this organizer owns.
	Calendars []Calendar `gorm:"foreignKey:OrganizerID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// ConfirmTimezone stores the client-supplied organizer timezone before the first temporal write.
func (userRecord *User) ConfirmTimezone(databaseConnection *gorm.DB, timezone Timezone) error {
	if userRecord.ID == "" {
		return ErrOrganizerIDRequired
	}
	if userRecord.Timezone != nil {
		return nil
	}
	timezoneName := timezone.String()
	if updateError := databaseConnection.Model(userRecord).Update("timezone", timezoneName).Error; updateError != nil {
		return fmt.Errorf("confirm timezone for organizer %s: %w", userRecord.ID, updateError)
	}
	userRecord.Timezone = &timezoneName
	return nil
}

// UpdateTimezone stores an explicit organizer timezone change.
func (userRecord *User) UpdateTimezone(databaseConnection *gorm.DB, timezone Timezone) error {
	if userRecord.ID == "" {
		return ErrOrganizerIDRequired
	}
	validatedTimezone, timezoneError := NewTimezone(timezone.String())
	if timezoneError != nil {
		return timezoneError
	}
	timezoneName := validatedTimezone.String()
	if updateError := databaseConnection.Model(userRecord).Update("timezone", timezoneName).Error; updateError != nil {
		return fmt.Errorf("update timezone for organizer %s: %w", userRecord.ID, updateError)
	}
	userRecord.Timezone = &timezoneName
	return nil
}

// BeforeCreate is a GORM hook executed before a new User record is inserted.
// It ensures that the User has a unique base62 ID generated if one is not already set.
func (userRecord *User) BeforeCreate(dbTransaction *gorm.DB) error {
	if validationError := userRecord.Validate(); validationError != nil {
		return validationError
	}
	if userRecord.ID == "" {
		generatedID, generateIDError := EnsureUniqueID(dbTransaction, config.TableUsers, GenerateBase62ID)
		if generateIDError != nil {
			return generateIDError
		}
		userRecord.ID = generatedID
	}
	return nil
}

// BeforeUpdate validates the organizer timezone when it is present.
func (userRecord *User) BeforeUpdate(*gorm.DB) error {
	return userRecord.Validate()
}

// Validate checks the optional organizer timezone.
func (userRecord *User) Validate() error {
	if userRecord.Timezone == nil {
		return nil
	}
	_, timezoneError := NewTimezone(*userRecord.Timezone)
	return timezoneError
}

// FindByEmail retrieves a single User from the database based on their unique email address.
// It populates the receiver 'userRecord' struct with the found data.
// Returns gorm.ErrRecordNotFound if no user with that email exists, or other errors for database issues.
func (userRecord *User) FindByEmail(databaseConnection *gorm.DB, emailAddress string) error {
	return databaseConnection.Where("email = ?", emailAddress).First(userRecord).Error
}

// FindByID retrieves a single User from the database based on their primary key (ID).
// It populates the receiver 'userRecord' struct with the found data.
// Returns gorm.ErrRecordNotFound if no user with that ID exists, or other errors for database issues.
func (userRecord *User) FindByID(databaseConnection *gorm.DB, userIdentifier string) error {
	return databaseConnection.Where("id = ?", userIdentifier).First(userRecord).Error
}

// Create inserts the current User struct instance (the receiver 'userRecord') as a new record into the database.
// Triggers the BeforeCreate hook to generate an ID if necessary.
// Returns an error if the database insertion fails.
func (userRecord *User) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(userRecord).Error
}

// Update updates the existing User record in the database corresponding to the receiver 'userRecord' struct's ID.
// Updates all fields based on the current values in the struct.
// Returns an error if the database update fails.
func (userRecord *User) Update(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(userRecord).Error
}

// UpsertUser finds a user by email or creates a new one if not found.
// If the user exists, it updates their Name and Picture if the provided values differ.
func UpsertUser(
	databaseConnection *gorm.DB,
	emailAddress string,
	fullName string,
	pictureURL string,
) (*User, error) {
	var existingUser User

	findResult := databaseConnection.First(&existingUser, "email = ?", emailAddress)

	if findResult.Error != nil {
		if errors.Is(findResult.Error, gorm.ErrRecordNotFound) {
			log.Printf("User with email %s not found, creating new user", emailAddress)
			newUser := User{
				Email:   emailAddress,
				Name:    fullName,
				Picture: pictureURL,
			}

			if createError := databaseConnection.Create(&newUser).Error; createError != nil {
				log.Printf("Error creating user: %v", createError)
				return nil, createError
			}
			log.Printf("User created successfully with ID: %s", newUser.ID)
			return &newUser, nil
		}
		log.Printf("Database error when finding user: %v", findResult.Error)
		return nil, findResult.Error
	}

	log.Printf("Found existing user with ID: %s", existingUser.ID)

	// Track if any fields need updating
	fieldsUpdated := false

	if existingUser.Name != fullName {
		existingUser.Name = fullName
		fieldsUpdated = true
	}

	if existingUser.Picture != pictureURL {
		existingUser.Picture = pictureURL
		fieldsUpdated = true
	}

	// Only update if fields have changed
	if fieldsUpdated {
		if updateError := databaseConnection.Save(&existingUser).Error; updateError != nil {
			log.Printf("Error updating user: %v", updateError)
			return nil, updateError
		}
		log.Printf("User updated successfully")
	}

	return &existingUser, nil
}
