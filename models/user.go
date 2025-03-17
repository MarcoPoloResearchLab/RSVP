package models

import (
	"errors"
	"log"
	"gorm.io/gorm"
)

type User struct {
	BaseModel
	Email   string  `gorm:"uniqueIndex;size:255;not null"`
	Name    string  `gorm:"size:255"`
	Picture string  `gorm:"size:512"`
	Events  []Event `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// BeforeCreate hook to generate a unique base62 ID
func (user *User) BeforeCreate(databaseTransaction *gorm.DB) error {
	if user.ID == "" {
		generatedID, generateError := EnsureUniqueID(databaseTransaction, "users", GenerateBase62ID)
		if generateError != nil {
			return generateError
		}
		user.ID = generatedID
	}
	return nil
}

// FindByEmail searches for a user by their email.
func (user *User) FindByEmail(databaseConnection *gorm.DB, email string) error {
	return databaseConnection.Where("email = ?", email).First(user).Error
}

// FindByID searches for a user by their ID.
func (user *User) FindByID(databaseConnection *gorm.DB, userID string) error {
	return databaseConnection.Where("id = ?", userID).First(user).Error
}

// Create inserts a new user into the database.
func (user *User) Create(databaseConnection *gorm.DB) error {
	return databaseConnection.Create(user).Error
}

// Save updates an existing user in the database.
func (user *User) Save(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(user).Error
}

// LoadWithEvents preloads the user's events.
func (user *User) LoadWithEvents(databaseConnection *gorm.DB, userID string) error {
	return databaseConnection.Preload("Events").Where("id = ?", userID).First(user).Error
}

// UpsertUser creates or updates a user record based on the email.
// It logs detailed information about the process for debugging.
func UpsertUser(
	databaseConnection *gorm.DB,
	email string,
	name string,
	picture string,
) (*User, error) {
	var user User

	// Try to find the user by email
	result := databaseConnection.First(&user, "email = ?", email)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// User not found, create a new one
			log.Printf("User with email %s not found, creating new user", email)
			user = User{
				Email:   email,
				Name:    name,
				Picture: picture,
			}
			
			// Create the user
			if createError := databaseConnection.Create(&user).Error; createError != nil {
				log.Printf("Error creating user: %v", createError)
				return nil, createError
			}
			
			// Verify the user was created by fetching it again
			var verifyUser User
			if verifyError := databaseConnection.First(&verifyUser, "email = ?", email).Error; verifyError != nil {
				log.Printf("Warning: User was created but could not be verified: %v", verifyError)
			} else {
				log.Printf("User created successfully with ID: %s", verifyUser.ID)
			}
		} else {
			// Some other database error
			log.Printf("Database error when finding user: %v", result.Error)
			return nil, result.Error
		}
	} else {
		// Existing user, update fields if changed
		log.Printf("Found existing user with ID: %s", user.ID)
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
			log.Printf("Updating user details for ID: %s", user.ID)
			if saveError := databaseConnection.Save(&user).Error; saveError != nil {
				log.Printf("Error updating user: %v", saveError)
				return nil, saveError
			}
		}
	}

	return &user, nil
}
