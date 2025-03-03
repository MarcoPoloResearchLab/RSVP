package config

import (
	"html/template"
	"log"

	"github.com/temirov/GAuss/pkg/gauss"
	"gorm.io/gorm"
)

const (
	WebRoot      = "/"
	WebGenerate  = "/generate"
	WebThankYou  = "/thankyou"
	WebResponses = "/responses"
	WebRSVP      = "/rsvp"
	WebSubmit    = "/submit"
)

// App holds shared resources for the application.
type App struct {
	Database    *gorm.DB
	Templates   *template.Template
	Logger      *log.Logger
	AuthService *gauss.Service
}
