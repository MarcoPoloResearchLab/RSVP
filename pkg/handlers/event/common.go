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
	AppTitle                  string
	ButtonAddVenue            string
	ButtonCancelEdit          string
	ButtonCreateNewVenue      string
	ButtonDeleteVenue         string
	ButtonUpdateEvent         string
	ButtonDeleteEvent         string
	CurrentDuration           string
	EventList                 []StatisticsData
	EventsManagerLabel        string
	FormattedStartTime        string
	LabelAddVenue             string
	LabelDuration             string
	LabelEventDescription     string
	LabelEventTitle           string
	LabelSelectVenue          string
	LabelStartTime            string
	LabelVenueAddress         string
	LabelVenueCapacity        string
	LabelVenueDescription     string
	LabelVenueDetails         string
	LabelVenueEmail           string
	LabelVenueFormTitle       string
	LabelVenueName            string
	LabelVenuePhone           string
	LabelVenueWebsite         string
	OptionCreateNewVenue      string
	OptionNoVenue             string
	ParamNameDescription      string
	ParamNameDuration         string
	ParamNameEventID          string
	ParamNameMethodOverride   string
	ParamNameStartTime        string
	ParamNameTitle            string
	ParamNameVenueAddress     string
	ParamNameVenueCapacity    string
	ParamNameVenueDescription string
	ParamNameVenueEmail       string
	ParamNameVenueID          string
	ParamNameVenueName        string
	ParamNameVenuePhone       string
	ParamNameVenueWebsite     string
	RSVPManagerLabel          string
	SelectedItemForEdit       *EnhancedEventData
	URLForEventActions        string
	URLForEventEdit           string
	URLForRSVPListBase        string
	URLForRSVPManager         string
	URLForVenueManager        string
	URLForVenues              string
	UserReusedVenues          []models.Venue
	VenueManagerLabel         string
	VenueSelectCreateNewValue string
}

// EditViewData is used specifically for the event edit view.
type EditViewData struct {
	ListViewData
	ActionAddExistingVenue string
	ActionCreateNewVenue   string
}
