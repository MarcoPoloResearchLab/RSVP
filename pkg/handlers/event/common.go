// Package event provides HTTP handler logic for event-related operations.
package event

import (
	"time"

	"github.com/tyemirov/RSVP/models"
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

// EnhancedEventData holds an event together with derived values.
type EnhancedEventData struct {
	Event                     models.Event
	CalculatedDurationInHours float64
	SelectedVenueID           string
}

// ListViewData is passed to the main “events” view template.
type ListViewData struct {
	/* view / app chrome */
	AppTitle           string
	EventsManagerLabel string
	VenueManagerLabel  string
	RSVPManagerLabel   string

	/* navigation URLs */
	URLForEventActions string
	URLForRSVPListBase string
	URLForRSVPManager  string
	URLForVenues       string

	/* event & venue data */
	EventList           []StatisticsData
	SelectedItemForEdit *EnhancedEventData
	UserReusedVenues    []models.Venue

	/* form/input helpers */
	ParamNameEventID          string
	ParamNameVenueID          string
	ParamNameTitle            string
	ParamNameDescription      string
	ParamNameStartTime        string
	ParamNameDuration         string
	ParamNameMethodOverride   string
	ParamNameVenueName        string
	ParamNameVenueAddress     string
	ParamNameVenueDescription string
	ParamNameVenueCapacity    string
	ParamNameVenuePhone       string
	ParamNameVenueEmail       string
	ParamNameVenueWebsite     string

	/* labels / buttons / options */
	LabelEventTitle       string
	LabelEventDescription string
	LabelStartTime        string
	LabelDuration         string
	LabelSelectVenue      string
	LabelAddVenue         string
	LabelVenueDetails     string
	LabelVenueName        string
	LabelVenueAddress     string
	LabelVenueDescription string
	LabelVenueCapacity    string
	LabelVenuePhone       string
	LabelVenueEmail       string
	LabelVenueWebsite     string

	ButtonCancelEdit     string
	ButtonAddVenue       string
	ButtonCreateNewVenue string
	ButtonUpdateEvent    string
	ButtonDeleteEvent    string
	ButtonDeleteVenue    string

	OptionNoVenue             string
	OptionCreateNewVenue      string
	VenueSelectCreateNewValue string

	/* misc */
	FormattedStartTime string
	CurrentDuration    string
}

type EditViewData struct {
	ListViewData
	ActionAddExistingVenue string
	ActionCreateNewVenue   string
}
