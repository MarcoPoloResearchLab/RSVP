package config

const (
	GoogleCalendarAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleCalendarListEndpoint          = "https://www.googleapis.com/calendar/v3/users/me/calendarList"
	GoogleCalendarEventsEndpoint        = "https://www.googleapis.com/calendar/v3/calendars"
	GoogleCalendarTokenEndpoint         = "https://oauth2.googleapis.com/token"
	GoogleCalendarListReadonlyScope     = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	GoogleCalendarEventsReadonlyScope   = "https://www.googleapis.com/auth/calendar.events.readonly"
	CalendarConnectionCallbackPath      = WebCalendarConnectionCallbacksGoogle
	IdempotencyKeyHeader                = "Idempotency-Key"
)

const (
	WebRoot                              = "/"
	WebCalendars                         = "/calendars/"
	WebAttentionPolicies                 = "/attention-policies/"
	WebCalendarAuthorizationRequests     = "/calendar-authorization-requests/"
	WebCalendarConnectionCallbacksGoogle = "/calendar-connection-callbacks/google/"
	WebCalendarConnections               = "/calendar-connections/"
	WebCalendarSyncs                     = "/calendar-syncs/"
	WebEvents                            = "/events/"
	WebDerivedMarkerRules                = "/derived-marker-rules/"
	WebHorizon                           = "/horizon/"
	WebIngestionDrafts                   = "/ingestion-drafts/"
	WebNaturalLanguageIngestion          = "/natural-language-ingestion/"
	WebLanes                             = "/lanes/"
	WebProbes                            = "/probes/"
	WebStatic                            = "/static/"
	WebRSVPs                             = "/rsvps/"
	WebRSVPQR                            = "/rsvps/qr/"
	WebResponse                          = "/response/"
	WebResponseThankYou                  = "/response/thankyou"
	WebVenues                            = "/venues/"
)

const (
	TemplateEvents    = "events"
	TemplateHorizon   = "horizon"
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
	HorizonStylesPath = WebStatic + "horizon.css"
	HorizonScriptPath = WebStatic + "horizon.js"
)

const (
	EventIDParam              = "event_id"
	AnchorEventIDParam        = "anchor_event_id"
	CalendarIDParam           = "calendar_id"
	HorizonEndParam           = "end"
	HorizonStartParam         = "start"
	RSVPIDParam               = "rsvp_id"
	VenueIDParam              = "venue_id"
	NameParam                 = "name"
	TitleParam                = "title"
	DescriptionParam          = "description"
	StartTimeParam            = "start_time"
	TimezoneParam             = "timezone"
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
	DefaultDBName                      = "rsvps.db"
	TableAttentionPolicies             = "attention_policies"
	TableCalendarAuthorizationRequests = "calendar_authorization_requests"
	TableCalendarConnections           = "calendar_connections"
	TableCalendars                     = "calendars"
	TableEvents                        = "events"
	TableDerivedMarkerRules            = "derived_marker_rules"
	TableDerivedMarkers                = "derived_markers"
	TableEventSeries                   = "event_series"
	TableExternalEventLinks            = "external_event_links"
	TableExternalEventSeriesLinks      = "external_event_series_links"
	TableCalendarSyncs                 = "calendar_syncs"
	TableLanes                         = "lanes"
	TableIdempotencyRecords            = "idempotency_records"
	TableIngestionDrafts               = "ingestion_drafts"
	TableDraftConfirmations            = "draft_confirmations"
	TableDraftDerivedMarkerRules       = "draft_derived_marker_rules"
	TableProbes                        = "probes"
	TableRSVPs                         = "rsvps"
	TableSourceCalendarMappings        = "source_calendar_mappings"
	TableUsers                         = "users"
	TableVenues                        = "venues"
)

const (
	ResourceNameEvent    = "Event"
	ResourceNameCalendar = "Calendar"
	ResourceNameHorizon  = "Horizon"
	ResourceNameLane     = "Lane"
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
	LabelTimezone         = "Timezone"
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
	AttentionClockInterval        = 60 * 1e9
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
