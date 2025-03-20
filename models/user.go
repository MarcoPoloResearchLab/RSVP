// Package models contains the database models for the RSVP system.
package models

import (
	"errors"
	"log"

	"gorm.io/gorm"
)

// User is the persistent record for an authenticated user.
type User struct {
	BaseModel
	Email   string  `gorm:"uniqueIndex;size:255;not null"`
	Name    string  `gorm:"size:255"`
	Picture string  `gorm:"size:512"`
	Events  []Event `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// BeforeCreate is a GORM hook that generates a unique base62 ID for this User.
func (userRecord *User) BeforeCreate(databaseTransaction *gorm.DB) error {
	if userRecord.ID == "" {
		generatedID, generateIDError := EnsureUniqueID(databaseTransaction, "users", GenerateBase62ID)
		if generateIDError != nil {
			return generateIDError
		}
		userRecord.ID = generatedID
	}
	return nil
}

// FindByEmail searches for a user by their email.
func (userRecord *User) FindByEmail(databaseConnection *gorm.DB, emailAddress string) error {
	return databaseConnection.Where("email = ?", emailAddress).First(userRecord).Error
}

// FindByID searches for a user by their ID.
func (userRecord *User) FindByID(databaseConnection *gorm.DB, userIdentifier string) error {
	return databaseConnection.Where("id = ?", userIdentifier).First(userRecord).Error
}

// Create inserts a new user into the database.
func (userRecord *User) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(userRecord).Error
}

// Save updates an existing user in the database.
func (userRecord *User) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(userRecord).Error
}

// UpsertUser creates or updates a user record based on the email.
func UpsertUser(
	databaseConnection *gorm.DB,
	emailAddress string,
	fullName string,
	pictureURL string,
) (*User, error) {
	var existingUser User

	// Try to find the user by email
	findResult := databaseConnection.First(&existingUser, "email = ?", emailAddress)
	if findResult.Error != nil {
		if errors.Is(findResult.Error, gorm.ErrRecordNotFound) {
			// User not found, create a new one
			log.Printf("User with email %s not found, creating new user", emailAddress)
			existingUser = User{
				Email:   emailAddress,
				Name:    fullName,
				Picture: pictureURL,
			}

			if createError := databaseConnection.Create(&existingUser).Error; createError != nil {
				log.Printf("Error creating user: %v", createError)
				return nil, createError
			}

			// Verify the user was created by fetching it again
			var verificationUser User
			if verifyError := databaseConnection.First(&verificationUser, "email = ?", emailAddress).Error; verifyError != nil {
				log.Printf("Warning: User was created but could not be verified: %v", verifyError)
			} else {
				log.Printf("User created successfully with ID: %s", verificationUser.ID)
			}
		} else {
			// Some other database error
			log.Printf("Database error when finding user: %v", findResult.Error)
			return nil, findResult.Error
		}
	} else {
		// Existing user, update fields if changed
		log.Printf("Found existing user with ID: %s", existingUser.ID)
		fieldsUpdated := false
		if existingUser.Name != fullName {
			existingUser.Name = fullName
			fieldsUpdated = true
		}
		if existingUser.Picture != pictureURL {
			existingUser.Picture = pictureURL
			fieldsUpdated = true
		}
		if fieldsUpdated {
			log.Printf("Updating user details for ID: %s", existingUser.ID)
			if saveError := databaseConnection.Save(&existingUser).Error; saveError != nil {
				log.Printf("Error updating user: %v", saveError)
				return nil, saveError
			}
		}
	}

	return &existingUser, nil
}
