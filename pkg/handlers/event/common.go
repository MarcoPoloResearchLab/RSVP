// Package event provides HTTP handler logic for event-related operations.
package event

import (
	"time"

	"github.com/temirov/RSVP/models"
)

// StatisticsData holds event statistics.
type StatisticsData struct {
	ID                string
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	VenueName         string
	RSVPCount         int
	RSVPAnsweredCount int
}

// EnhancedEventData holds an event along with calculated values.
type EnhancedEventData struct {
	Event                     models.Event
	CalculatedDurationInHours float64
	SelectedVenueID           string
}

// ListViewData holds common fields used in event-related views.
type ListViewData struct {
	EventList                 []StatisticsData
	SelectedItemForEdit       *EnhancedEventData
	UserReusedVenues          []models.Venue
	URLForEventActions        string
	URLForEventEdit           string
	URLForRSVPListBase        string
	URLForVenues              string
	ParamNameEventID          string
	ParamNameVenueID          string
	ParamNameTitle            string
	ParamNameDescription      string
	ParamNameStartTime        string
	ParamNameDuration         string
	ParamNameMethodOverride   string
	ParamNameVenueName        string
	ParamNameVenueAddress     string
	ParamNameVenueCapacity    string
	ParamNameVenuePhone       string
	ParamNameVenueEmail       string
	ParamNameVenueWebsite     string
	ParamNameVenueDescription string
	VenueSelectCreateNewValue string
	ButtonCancelEdit          string
	LabelEventTitle           string
	LabelEventDescription     string
	LabelStartTime            string
	LabelDuration             string
	LabelVenueDetails         string
	ButtonDeleteVenue         string
	ButtonAddVenue            string
	ButtonCreateNewVenue      string
	LabelAddVenue             string
	LabelSelectVenue          string
	OptionNoVenue             string
	OptionCreateNewVenue      string
	LabelVenueFormTitle       string
	LabelVenueName            string
	LabelVenueAddress         string
	LabelVenueDescription     string
	LabelVenueCapacity        string
	LabelVenuePhone           string
	LabelVenueEmail           string
	LabelVenueWebsite         string
	ButtonUpdateEvent         string
	FormattedStartTime        string
	CurrentDuration           string
	ShowAddVenueSubform       bool
	AppTitle                  string
	RSVPManagerLabel          string
	URLForRSVPManager         string
	VenueManagerLabel         string
	URLForVenueManager        string
}

// EditViewData is used specifically for the event edit view.
type EditViewData struct {
	ListViewData
	ActionAddExistingVenue string
	ActionCreateNewVenue   string
}
