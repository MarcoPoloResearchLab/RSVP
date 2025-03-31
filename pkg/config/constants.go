package config

import "time"

// Constants defining common web paths used throughout the application.
const (
	WebRoot             = "/"
	WebEvents           = "/events/"
	WebRSVPs            = "/rsvps/"
	WebRSVPQR           = "/rsvps/qr/"
	WebResponse         = "/response/"
	WebResponseThankYou = "/response/thankyou"
)

// Constants defining template base names used for looking up precompiled templates
// by the main template loader.
const (
	TemplateEvents    = "events"
	TemplateRSVP      = "rsvp"
	TemplateRSVPs     = "rsvps"
	TemplateResponse  = "response"
	TemplateThankYou  = "thankyou"
	TemplateExtension = ".tmpl"
	TemplateLayout    = "layout"    // Name of the main layout template
	TemplateLanding   = "landing"   // Name of the standalone landing page template
	TemplatesDir      = "templates" // Directory where application templates are stored.
	PartialsDir       = "partials"  // Subdirectory for partial templates
)

// Constants defining parameter names used in HTTP requests (query and form values).
const (
	EventIDParam        = "event_id"
	RSVPIDParam         = "rsvp_id"
	NameParam           = "name"
	TitleParam          = "title"
	DescriptionParam    = "description"
	StartTimeParam      = "start_time"
	DurationParam       = "duration"
	ResponseParam       = "response"
	ExtraGuestsParam    = "extra_guests" // Added for clarity, potentially used in Phase 2
	MethodOverrideParam = "_method"
	ErrorQueryParam     = "error" // Used by GAuss on redirect
)

// Database related constants.
const (
	DefaultDBName = "rsvps.db"
	TableEvents   = "events"
	TableRSVPs    = "rsvps"
	TableUsers    = "users"
)

// Resource names used for logging and potentially UI messages.
const (
	ResourceNameEvent    = "Event"
	ResourceNameRSVP     = "RSVP"
	ResourceNameRSVPQR   = "RSVP QR Code"
	ResourceNameResponse = "Response"
	ResourceNameThankYou = "Thank You Page"
	ResourceNameUser     = "User"
)

// ID generation constants.
const (
	// Base62Chars is the charset used for base62 encoding (0-9, A-Z, a-z).
	Base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base36Chars is the charset used for base36 encoding (0-9, A-Z).
	Base36Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// IDLength is the standard length for IDs.
	IDLength = 8
	// MaxIDGenerationAttempts is the maximum number of attempts for unique ID generation.
	MaxIDGenerationAttempts = 10
)

// Validation constants.
const (
	MaxTitleLength     = 255
	MaxNameLength      = 100
	MaxGuestCount      = 4
	MinEventDuration   = 1                  // hours
	MaxEventDuration   = 4                  // hours
	TimeLayoutHTMLForm = "2006-01-02T15:04" // Format used by <input type="datetime-local">
)

// RSVP Response values.
const (
	RSVPResponsePending      = "Pending"
	RSVPResponseYesPrefix    = "Yes"
	RSVPResponseNo           = "No"
	RSVPResponseNoCommaZero  = "No,0" // Standardized 'No' response in DB
	RSVPResponseYesBase      = "Yes," // Base for constructing Yes responses
	RSVPResponseYesJustMe    = "Yes,0"
	RSVPResponseYesPlusOne   = "Yes,1"
	RSVPResponseYesPlusTwo   = "Yes,2"
	RSVPResponseYesPlusThree = "Yes,3"
	RSVPResponseYesPlusFour  = "Yes,4"
)

// Server configuration constants.
const (
	ServerHTTPPort                = 8080
	ServerHTTPAddress             = "0.0.0.0"
	ServerGracefulShutdownTimeout = 10 * time.Second
)

// Logging constants.
const (
	LogPrefixApp = "[APP] "
)

// Other constants.
const (
	RSVPCodeValidationRegexPattern = `^[0-9a-zA-Z]{1,8}$`
)
