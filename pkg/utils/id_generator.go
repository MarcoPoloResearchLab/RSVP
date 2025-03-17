package utils

import (
	"crypto/rand"
	"errors"
	"math/big"

	"gorm.io/gorm"
)

const (
	// Base62Chars contains all characters used for base62 encoding (0-9, A-Z, a-z)
	Base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base36Chars contains all characters used for base36 encoding (0-9, A-Z)
	Base36Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// DefaultIDLength is the standard length for IDs
	DefaultIDLength = 8
	// MaxAttempts is the maximum number of attempts to generate a unique ID
	MaxAttempts = 10
)

// IDGenerator configures ID generation for a model
type IDGenerator struct {
	Charset   string
	Length    int
	TableName string
}

// NewIDGenerator creates a new IDGenerator with the given configuration
func NewIDGenerator(charset string, length int, tableName string) *IDGenerator {
	return &IDGenerator{
		Charset:   charset,
		Length:    length,
		TableName: tableName,
	}
}

// NewBase62IDGenerator creates a new IDGenerator with Base62 charset
func NewBase62IDGenerator(tableName string) *IDGenerator {
	return NewIDGenerator(Base62Chars, DefaultIDLength, tableName)
}

// NewBase36IDGenerator creates a new IDGenerator with Base36 charset
func NewBase36IDGenerator(tableName string) *IDGenerator {
	return NewIDGenerator(Base36Chars, DefaultIDLength, tableName)
}

// Generate generates a random ID using the configured charset and length
func (g *IDGenerator) Generate() (string, error) {
	result := make([]byte, g.Length)
	charsetLength := big.NewInt(int64(len(g.Charset)))

	for i := 0; i < g.Length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", err
		}
		result[i] = g.Charset[randomIndex.Int64()]
	}

	return string(result), nil
}

// GenerateUnique generates a unique ID and verifies it doesn't exist in the database
func (g *IDGenerator) GenerateUnique(db *gorm.DB) (string, error) {
	for attempt := 0; attempt < MaxAttempts; attempt++ {
		// Generate a new ID
		id, err := g.Generate()
		if err != nil {
			return "", err
		}

		// Check if the ID already exists
		var exists bool
		err = db.Table(g.TableName).
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

// GenerateBase36ID generates a random base36 ID of the specified length
// This is a wrapper around the existing Base36Encode function for compatibility
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
