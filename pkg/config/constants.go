package config

const (
	WebRoot             = "/"
	WebEvents           = "/events/"
	WebRSVPs            = "/rsvps/"
	WebRSVPQR           = "/rsvps/qr/"
	WebResponse         = "/response/"
	WebResponseThankYou = "/response/thankyou"
	WebVenues           = "/venues/"
)

const (
	TemplateEvents    = "events"
	TemplateRSVP      = "rsvp"
	TemplateRSVPs     = "rsvps"
	TemplateResponse  = "response"
	TemplateThankYou  = "thankyou"
	TemplateVenues    = "venues"
	TemplateExtension = ".tmpl"
	TemplateLayout    = "layout"
	TemplateLanding   = "landing"
	TemplatesDir      = "templates"
	PartialsDir       = "partials"
)

const (
	EventIDParam              = "event_id"
	RSVPIDParam               = "rsvp_id"
	VenueIDParam              = "venue_id"
	NameParam                 = "name"
	TitleParam                = "title"
	DescriptionParam          = "description"
	StartTimeParam            = "start_time"
	DurationParam             = "duration"
	ResponseParam             = "response"
	ExtraGuestsParam          = "extra_guests"
	MethodOverrideParam       = "_method"
	ErrorQueryParam           = "error"
	VenueNameParam            = "venue_name"
	VenueAddressParam         = "venue_address"
	VenueCapacityParam        = "venue_capacity"
	VenuePhoneParam           = "venue_phone"
	VenueEmailParam           = "venue_email"
	VenueWebsiteParam         = "venue_website"
	VenueDescriptionParam     = "venue_description"
	VenueSelectCreateNewValue = "__CREATE_NEW__"
	ActionQueryParam          = "action"
	ActionManageVenue         = "manage_venue"
)

const (
	ActionParam            = ActionQueryParam
	ErrMsgTransactionStart = "Failed to start transaction"
	ErrMsgEventNotFound    = "Event not found"
)

const (
	DefaultDBName = "rsvps.db"
	TableEvents   = "events"
	TableRSVPs    = "rsvps"
	TableUsers    = "users"
	TableVenues   = "venues"
)

const (
	ResourceNameEvent    = "Event"
	ResourceNameRSVP     = "RSVP"
	ResourceNameRSVPQR   = "RSVP QR Code"
	ResourceNameResponse = "Response"
	ResourceNameThankYou = "Thank You Page"
	ResourceNameUser     = "User"
	ResourceNameVenue    = "Venue"
)

const (
	Base62Chars             = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	Base36Chars             = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	IDLength                = 8
	MaxIDGenerationAttempts = 10
)

const (
	MaxTitleLength     = 255
	MaxNameLength      = 100
	MaxGuestCount      = 4
	MinEventDuration   = 1
	MaxEventDuration   = 4
	TimeLayoutHTMLForm = "2006-01-02T15:04"
	MaxVenueNameLength = 200
)

const (
	ErrMsgInvalidFormData        = "Invalid form data"
	ActionUpdateEventDetails     = "update_event_details"
	ErrMsgInvalidStartTimeFormat = "Invalid start time format"
	ErrMsgEventUpdate            = "Failed to update event"
	ActionRemoveVenue            = "remove_venue"
	ErrMsgVenueRemoval           = "Failed to remove venue"
	ActionShowAddVenue           = "show_add_venue"
	ActionAddExistingVenue       = "add_existing_venue"
	ErrMsgVenuePermission        = "You do not have permission to use the selected venue"
	ErrMsgVenueAssociation       = "Failed to associate venue"
	ActionCreateNewVenue         = "create_new_venue"
	ErrMsgVenueCreation          = "Failed to create new venue"
	ErrMsgUnknownAction          = "Unknown action"
	ButtonCancelEdit             = "Cancel Edit"
)

const (
	RSVPResponsePending      = "Pending"
	RSVPResponseYesPrefix    = "Yes"
	RSVPResponseNo           = "No"
	RSVPResponseNoCommaZero  = "No,0"
	RSVPResponseYesBase      = "Yes,"
	RSVPResponseYesJustMe    = "Yes,0"
	RSVPResponseYesPlusOne   = "Yes,1"
	RSVPResponseYesPlusTwo   = "Yes,2"
	RSVPResponseYesPlusThree = "Yes,3"
	RSVPResponseYesPlusFour  = "Yes,4"
)

const (
	ButtonAddVenue        = "Add Venue"
	ButtonCreateVenue     = "Create New Venue"
	ButtonDeleteEvent     = "Delete Event"
	ButtonUpdateEvent     = "Update Event"
	ButtonDeleteVenue     = "Delete Venue"
	ButtonUpdateVenue     = "Update Venue"
	LabelAddVenue         = "Add Venue"
	LabelDuration         = "Duration"
	LabelEventDescription = "Event Description"
	LabelEventTitle       = "Event Title"
	LabelSelectVenue      = "Select Venue"
	LabelStartTime        = "Start Time"
	LabelVenueAddress     = "Venue Address"
	LabelVenueCapacity    = "Venue Capacity"
	LabelVenueDescription = "Venue Description"
	LabelVenueDetails     = "Venue Details"
	LabelVenueEmail       = "Venue Email"
	LabelVenueFormTitle   = "Venue Information"
	LabelVenueName        = "Venue Name"
	LabelVenuePhone       = "Venue Phone"
	LabelVenueWebsite     = "Venue Website"
	OptionCreateNewVenue  = "-- Create New Venue --"
	OptionNoVenue         = "-- No Venue --"
)

const (
	ServerHTTPPort                = 8080
	ServerHTTPAddress             = "0.0.0.0"
	ServerGracefulShutdownTimeout = 10 * 1e9
)

const (
	LogPrefixApp = "[APP] "
)

const (
	RSVPCodeValidationRegexPattern = `^[0-9a-zA-Z]{1,8}$`
)

const (
	ContextKeyUser = "user"
	DatabaseError  = "database_error"
)

const (
	ResourceLabelEventManager = "Events"
	ResourceLabelVenueManager = "Venues"
	AppTitle                  = "RSVP Manager"
	LabelWelcome              = "Welcome,"
	LabelSignOut              = "Sign Out"
	LabelNotSignedIn          = "Not signed in"
)

const (
	MapsSearchBaseURL = "https://www.google.com/maps/search/?api=1&query="
)
