package venue

import (
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

type ListViewData struct {
	VenueList                 []models.Venue
	SelectedItemForEdit       *models.Venue
	URLForVenueActions        string
	URLForVenues              string
	ParamNameMethodOverride   string
	ParamNameVenueID          string
	ParamNameVenueName        string
	ParamNameVenueAddress     string
	ParamNameVenueDescription string
	ParamNameVenueCapacity    string
	ParamNameVenuePhone       string
	ParamNameVenueEmail       string
	ParamNameVenueWebsite     string
	ButtonCancelEdit          string
	ButtonUpdateVenue         string
	ButtonDeleteVenue         string
	LabelVenueName            string
	LabelVenueAddress         string
	LabelVenueDescription     string
	LabelVenueCapacity        string
	LabelVenuePhone           string
	LabelVenueEmail           string
	LabelVenueWebsite         string
	VenueManagerLabel         string
}

func NewListViewData(venueList []models.Venue, selectedVenue *models.Venue) ListViewData {
	return ListViewData{
		VenueList:                 venueList,
		SelectedItemForEdit:       selectedVenue,
		URLForVenueActions:        config.WebVenues,
		URLForVenues:              config.WebVenues,
		ParamNameMethodOverride:   config.MethodOverrideParam,
		ParamNameVenueID:          config.VenueIDParam,
		ParamNameVenueName:        config.VenueNameParam,
		ParamNameVenueAddress:     config.VenueAddressParam,
		ParamNameVenueDescription: config.VenueDescriptionParam,
		ParamNameVenueCapacity:    config.VenueCapacityParam,
		ParamNameVenuePhone:       config.VenuePhoneParam,
		ParamNameVenueEmail:       config.VenueEmailParam,
		ParamNameVenueWebsite:     config.VenueWebsiteParam,
		ButtonCancelEdit:          config.ButtonCancelEdit,
		ButtonUpdateVenue:         config.ButtonUpdateVenue,
		ButtonDeleteVenue:         config.ButtonDeleteVenue,
		LabelVenueName:            config.LabelVenueName,
		LabelVenueAddress:         config.LabelVenueAddress,
		LabelVenueDescription:     config.LabelVenueDescription,
		LabelVenueCapacity:        config.LabelVenueCapacity,
		LabelVenuePhone:           config.LabelVenuePhone,
		LabelVenueEmail:           config.LabelVenueEmail,
		LabelVenueWebsite:         config.LabelVenueWebsite,
		VenueManagerLabel:         config.ResourceLabelVenueManager,
	}
}
