package utils

import "strings"

// JoinStrings joins a slice of strings with a separator.
// Included here as it was referenced in base_handler.go.
func JoinStrings(elements []string, separator string) string {
	return strings.Join(elements, separator)
}

// You might want to add IsRecordNotFoundError here as well if not already present elsewhere
// import "errors"
// import "gorm.io/gorm"
// func IsRecordNotFoundError(err error) bool {
// 	return errors.Is(err, gorm.ErrRecordNotFound)
// }
