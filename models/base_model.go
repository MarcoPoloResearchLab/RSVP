package models

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"gorm.io/gorm"
)

// BaseModel replaces gorm.Model with a string ID
type BaseModel struct {
	ID        string         `gorm:"primaryKey;type:varchar(8);index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

const (
	// Base62Chars contains all characters used for base62 encoding (0-9, A-Z, a-z)
	Base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base36Chars contains all characters used for base36 encoding (0-9, A-Z)
	Base36Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// IDLength is the standard length for IDs
	IDLength = 8
	// MaxAttempts is the maximum number of attempts to generate a unique ID
	MaxAttempts = 10
)

// GenerateBase62ID generates a random base62 ID of the specified length
func GenerateBase62ID(length int) (string, error) {
	return generateRandomID(length, Base62Chars)
}

// GenerateBase36ID generates a random base36 ID of the specified length
func GenerateBase36ID(length int) (string, error) {
	return generateRandomID(length, Base36Chars)
}

// generateRandomID creates a random ID using the given character set
func generateRandomID(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLength := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", err
		}
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result), nil
}

// EnsureUniqueID checks if an ID exists in the database and generates a new one if needed
func EnsureUniqueID(db *gorm.DB, tableName string, generateFunc func(int) (string, error)) (string, error) {
	var id string
	var err error
	var exists bool

	for attempt := 0; attempt < MaxAttempts; attempt++ {
		// Generate a new ID
		id, err = generateFunc(IDLength)
		if err != nil {
			return "", err
		}

		// Check if the ID already exists
		err = db.Table(tableName).
			Select("count(*) > 0").
			Where("id = ?", id).
			Scan(&exists).Error

		if err != nil {
			return "", err
		}

		// If the ID doesn't exist, return it
		if !exists {
			return id, nil
		}
	}

	return "", errors.New("failed to generate a unique ID after maximum attempts")
}
