package models

import (
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

type Venue struct {
	BaseModel
	UserID      string `gorm:"not null;index"`
	Name        string `gorm:"not null"`
	Address     string
	Capacity    int
	Website     string
	Phone       string
	Email       string
	Description string
	Events      []Event `gorm:"foreignKey:VenueID"`
}

func (venue *Venue) GetTableName() string {
	return config.TableVenues
}

func (venue *Venue) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

func (venue *Venue) BeforeCreate(tx *gorm.DB) error {
	if err := venue.BaseModel.GenerateID(tx, venue); err != nil {
		return err
	}
	return venue.Validate()
}

func (venue *Venue) BeforeUpdate(tx *gorm.DB) error {
	return venue.Validate()
}

func (venue *Venue) Validate() error {
	if err := utils.ValidateVenueName(venue.Name); err != nil {
		return err
	}
	return nil
}

func (venue *Venue) FindByID(databaseConnection *gorm.DB, venueIdentifier string) error {
	return databaseConnection.Where("id = ?", venueIdentifier).First(venue).Error
}

// FindByIDAndOwner retrieves a Venue record by its identifier ensuring it belongs to the specified owner.
func (venue *Venue) FindByIDAndOwner(databaseConnection *gorm.DB, venueIdentifier string, ownerUserID string) error {
	return databaseConnection.Where("id = ? AND user_id = ?", venueIdentifier, ownerUserID).First(venue).Error
}

func (venue *Venue) Create(databaseConnection *gorm.DB) error {
	if venue.UserID == "" {
		return utils.ErrUserIDRequired
	}
	return databaseConnection.Create(venue).Error
}

func (venue *Venue) Update(databaseConnection *gorm.DB) error {
	return databaseConnection.Save(venue).Error
}

func FindVenuesByIDs(databaseConnection *gorm.DB, venueIDs []string) ([]Venue, error) {
	var venues []Venue
	if len(venueIDs) == 0 {
		return venues, nil
	}
	result := databaseConnection.Where("id IN ?", venueIDs).Order("name ASC").Find(&venues)
	return venues, result.Error
}

func FindVenuesByOwner(databaseConnection *gorm.DB, ownerID string) ([]Venue, error) {
	var venues []Venue
	err := databaseConnection.Where("user_id = ?", ownerID).Order("name ASC").Find(&venues).Error
	return venues, err
}

func (venue *Venue) Delete(db *gorm.DB) error {
	if err := db.Model(&Event{}).Where("venue_id = ?", venue.ID).Update("venue_id", gorm.Expr("NULL")).Error; err != nil {
		utils.NewLogger().Printf("WARN: Failed to disassociate events from venue %s during deletion: %v", venue.ID, err)
	}
	if err := db.Delete(venue).Error; err != nil {
		return err
	}
	return nil
}
