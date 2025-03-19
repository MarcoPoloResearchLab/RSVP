package models

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"gorm.io/gorm"
)

// BaseModel replaces gorm.Model with a string ID.
type BaseModel struct {
	ID        string `gorm:"primaryKey;type:varchar(8);index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

const (
	// Base62Chars contains all characters used for base62 encoding (0-9, A-Z, a-z).
	Base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base36Chars contains all characters used for base36 encoding (0-9, A-Z).
	Base36Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// IDLength is the standard length for IDs.
	IDLength = 8
	// MaxAttempts is the maximum number of attempts to generate a unique ID.
	MaxAttempts = 10
)

// GenerateBase62ID generates a random base62 ID of the specified length.
func GenerateBase62ID(desiredLength int) (string, error) {
	return generateRandomID(desiredLength, Base62Chars)
}

// GenerateBase36ID generates a random base36 ID of the specified length.
func GenerateBase36ID(desiredLength int) (string, error) {
	return generateRandomID(desiredLength, Base36Chars)
}

// generateRandomID creates a random ID using the given character set.
func generateRandomID(desiredLength int, characterSet string) (string, error) {
	randomResult := make([]byte, desiredLength)
	characterSetLength := big.NewInt(int64(len(characterSet)))

	for index := 0; index < desiredLength; index++ {
		randomIndex, randomIndexError := rand.Int(rand.Reader, characterSetLength)
		if randomIndexError != nil {
			return "", randomIndexError
		}
		randomResult[index] = characterSet[randomIndex.Int64()]
	}

	return string(randomResult), nil
}

// EnsureUniqueID checks if an ID exists in the database and generates a new one if needed.
func EnsureUniqueID(databaseConnection *gorm.DB, tableName string, generateFunc func(int) (string, error)) (string, error) {
	var generatedID string
	var generationError error
	var alreadyExists bool

	for attemptIndex := 0; attemptIndex < MaxAttempts; attemptIndex++ {
		// Generate a new ID
		generatedID, generationError = generateFunc(IDLength)
		if generationError != nil {
			return "", generationError
		}

		// Check if the ID already exists
		queryError := databaseConnection.Table(tableName).
			Select("count(*) > 0").
			Where("id = ?", generatedID).
			Scan(&alreadyExists).Error

		if queryError != nil {
			return "", queryError
		}

		// If the ID doesn't exist, return it
		if !alreadyExists {
			return generatedID, nil
		}
	}

	return "", errors.New("failed to generate a unique ID after maximum attempts")
}
