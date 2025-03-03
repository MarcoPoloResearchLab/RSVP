package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email   string  `gorm:"uniqueIndex;size:255;not null"`
	Name    string  `gorm:"size:255"`
	Picture string  `gorm:"size:512"`
	Events  []Event `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (user *User) FindByEmail(db *gorm.DB, email string) error {
	return db.Where("email = ?", email).First(user).Error
}

func (user *User) Create(db *gorm.DB) error {
	return db.Create(user).Error
}

func (user *User) Save(db *gorm.DB) error {
	return db.Save(user).Error
}

func (user *User) LoadWithEvents(db *gorm.DB, userID uint) error {
	return db.Preload("Events").First(user, userID).Error
}

func UpsertUser(
	db *gorm.DB,
	googleID string,
	email string,
	name string,
	picture string,
) (*User, error) {
	var user User

	result := db.First(&user, "google_id = ?", googleID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
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
		if user.Email != email {
			user.Email = email
			updated = true
		}
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
