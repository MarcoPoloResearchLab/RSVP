package config

import (
	"html/template"
	"log"

	"github.com/temirov/GAuss/pkg/gauss"
	"gorm.io/gorm"
)

const (
	WebRoot       = "/"
	WebGenerate   = "/generate"
	WebThankYou   = "/thankyou"
	WebResponses  = "/responses"
	WebRSVP       = "/rsvp"
	WebSubmit     = "/submit"
	WebRSVPs      = "/rsvps"
	WebUnderRSVPs = "/rsvps/"
	WebQRSuffix   = "/qr"
)

// Template names are defined as constants so they can be changed easily without altering logic.
const (
	TemplateIndex     = "index.html"
	TemplateResponses = "responses.html"
	TemplateGenerate  = "generate.html"
	TemplateThankYou  = "thankyou.html"
	TemplateRSVP      = "rsvp.html"
)

// App holds shared resources for the application.
type App struct {
	Database    *gorm.DB
	Templates   *template.Template
	Logger      *log.Logger
	AuthService *gauss.Service
}
