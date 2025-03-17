package utils

import (
	"errors"
	"gorm.io/gorm"
)

// Repository provides generic database operations for models
type Repository struct {
	DB *gorm.DB
}

// NewRepository creates a new repository with the given database connection
func NewRepository(db *gorm.DB) Repository {
	return Repository{DB: db}
}

// FindByID finds a record by its ID
func (r *Repository) FindByID(model interface{}, id string) error {
	return r.DB.Where("id = ?", id).First(model).Error
}

// FindByField finds a record by a specific field
func (r *Repository) FindByField(model interface{}, fieldName string, value interface{}) error {
	return r.DB.Where(fieldName+" = ?", value).First(model).Error
}

// Create inserts a new record into the database
func (r *Repository) Create(model interface{}) error {
	return r.DB.Create(model).Error
}

// Save updates an existing record in the database
func (r *Repository) Save(model interface{}) error {
	return r.DB.Save(model).Error
}

// Delete removes a record from the database
func (r *Repository) Delete(model interface{}) error {
	return r.DB.Delete(model).Error
}

// Preload loads associated models
func (r *Repository) Preload(model interface{}, id string, associations ...string) error {
	query := r.DB
	for _, association := range associations {
		query = query.Preload(association)
	}
	return query.Where("id = ?", id).First(model).Error
}

// Count returns the number of records matching the given conditions
func (r *Repository) Count(model interface{}, conditions ...interface{}) (int64, error) {
	var count int64
	tx := r.DB.Model(model)
	
	if len(conditions) > 0 {
		if len(conditions) == 1 {
			tx = tx.Where(conditions[0])
		} else {
			tx = tx.Where(conditions[0], conditions[1:]...)
		}
	}
	
	err := tx.Count(&count).Error
	return count, err
}

// IsNotFound checks if an error is a "record not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
