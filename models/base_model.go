// Package models contains the database models (structs corresponding to tables)
// and related database interaction logic for the RSVP system.
package models

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/temirov/RSVP/pkg/config" // Import config for constants
	"gorm.io/gorm"
)

// ErrFailedToGenerateUniqueID indicates that the system could not generate a unique ID
// within the maximum allowed attempts.
var ErrFailedToGenerateUniqueID = errors.New("failed to generate a unique ID after maximum attempts")

// BaseModel provides common fields for database models, including a string-based
// primary key (ID) and standard GORM timestamp/soft-delete fields.
type BaseModel struct {
	// ID is the unique primary key for the model, stored as an 8-character string.
	ID string `gorm:"primaryKey;type:varchar(8);index"`
	// CreatedAt records the time the record was created. Managed by GORM.
	CreatedAt time.Time
	// UpdatedAt records the time the record was last updated. Managed by GORM.
	UpdatedAt time.Time
	// DeletedAt records the time the record was soft-deleted. Managed by GORM.
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// GenerateBase62ID generates a random base62 (alphanumeric) ID of the specified length.
// It is typically used for generating User and Event IDs.
func GenerateBase62ID(desiredLength int) (string, error) {
	return generateRandomID(desiredLength, config.Base62Chars)
}

// GenerateBase36ID generates a random base36 (uppercase alphanumeric) ID of the specified length.
// It is typically used for generating RSVP IDs (codes).
func GenerateBase36ID(desiredLength int) (string, error) {
	return generateRandomID(desiredLength, config.Base36Chars)
}

// generateRandomID creates a random ID using the provided character set and length.
// It uses crypto/rand for secure random number generation.
func generateRandomID(desiredLength int, characterSet string) (string, error) {
	randomResult := make([]byte, desiredLength)
	characterSetLength := big.NewInt(int64(len(characterSet)))

	for index := 0; index < desiredLength; index++ {
		// Generate a random index within the bounds of the character set.
		randomIndex, randomIndexError := rand.Int(rand.Reader, characterSetLength)
		if randomIndexError != nil {
			// Return an empty string and the error if random number generation fails.
			return "", randomIndexError
		}
		// Assign the character at the random index to the result byte slice.
		randomResult[index] = characterSet[randomIndex.Int64()]
	}

	// Convert the byte slice to a string and return it.
	return string(randomResult), nil
}

// EnsureUniqueID checks if a generated ID already exists in the specified database table.
// If the first generated ID exists, it retries using the generateFunc up to MaxIDGenerationAttempts times.
// It returns a unique ID or an error if generation or database checks fail, or if no unique ID is found.
func EnsureUniqueID(databaseConnection *gorm.DB, tableName string, generateFunc func(int) (string, error)) (string, error) {
	var generatedID string
	var generationError error
	var idExists bool

	// Attempt to generate a unique ID up to the maximum allowed attempts.
	for attemptIndex := 0; attemptIndex < config.MaxIDGenerationAttempts; attemptIndex++ {
		// Generate a candidate ID using the provided generation function.
		generatedID, generationError = generateFunc(config.IDLength)
		if generationError != nil {
			return "", generationError // Return error if ID generation fails.
		}

		// Check if the generated ID already exists in the specified table.
		// This query checks for the existence efficiently without retrieving the full record.
		queryError := databaseConnection.Table(tableName).
			Select("count(*) > 0"). // Select a boolean indicating existence.
			Where("id = ?", generatedID).
			Scan(&idExists).Error // Scan the result into the idExists variable.
		if queryError != nil {
			return "", queryError // Return error if the database query fails.
		}

		// If the ID does not exist, it's unique. Return it.
		if !idExists {
			return generatedID, nil
		}
		// If the ID exists, the loop continues to the next attempt.
	}

	// If no unique ID was found after all attempts, return an error.
	return "", ErrFailedToGenerateUniqueID
}
