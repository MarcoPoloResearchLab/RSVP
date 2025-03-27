// File: pkg/handlers/rsvp/show.go
package rsvp

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url" // Import url package

	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// RSVPShowViewData holds data for the rsvp.tmpl (QR code display) view.
type RSVPShowViewData struct {
	RSVP      models.RSVP
	Event     models.Event
	QRCode    string // Base64 encoded PNG image data
	PublicURL string // Full URL for the public response page
	// Config values
	URLForRSVPList string
	ParamEventID   string
	ParamRSVPID    string
}

// ShowHandler handles GET requests to display the QR code page for a specific RSVP.
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "RSVP QR", config.WebRSVPQR)
	envConfig := config.NewEnvConfig(applicationContext.Logger)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		userSessionData, isAuthenticated := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)
		if !isAuthenticated {
			return
		}

		var currentUser models.User
		userFindErr := currentUser.FindByEmail(applicationContext.Database, userSessionData.UserEmail)
		if userFindErr != nil {
			if errors.Is(userFindErr, gorm.ErrRecordNotFound) {
				newUser, upsertErr := models.UpsertUser(applicationContext.Database, userSessionData.UserEmail, userSessionData.UserName, userSessionData.UserPicture)
				if upsertErr != nil {
					baseHandler.HandleError(httpResponseWriter, upsertErr, utils.DatabaseError, "Failed to create user record.")
					return
				}
				currentUser = *newUser
			} else {
				baseHandler.HandleError(httpResponseWriter, userFindErr, utils.DatabaseError, "Could not retrieve user info.")
				return
			}
		}

		rsvpID := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		if rsvpID == "" {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "RSVP ID is required to view the QR code.")
			return
		}
		if !handlers.ValidateRSVPCode(rsvpID) {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Invalid RSVP ID format.")
			return
		}

		var rsvpRecord models.RSVP
		findRsvpErr := rsvpRecord.FindByCode(applicationContext.Database, rsvpID)
		if findRsvpErr != nil {
			if errors.Is(findRsvpErr, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.NotFoundError, "The specified RSVP was not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}

		var eventRecord models.Event
		eventFindErr := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID)
		if eventFindErr != nil {
			applicationContext.Logger.Printf("ERROR: Could not find parent event %s for RSVP %s during QR code generation", rsvpRecord.EventID, rsvpID)
			baseHandler.HandleError(httpResponseWriter, eventFindErr, utils.NotFoundError, "Could not find the parent event for this RSVP.")
			return
		}

		if eventRecord.UserID != currentUser.ID {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ForbiddenError, "You do not have permission to view the QR code for this RSVP.")
			return
		}

		// --- Corrected URL Construction ---
		var finalPublicURL *url.URL
		var constructErr error

		// Start with the base application URL if available
		if envConfig.AppBaseURL != "" {
			finalPublicURL, constructErr = url.Parse(envConfig.AppBaseURL)
			if constructErr != nil {
				applicationContext.Logger.Printf("CRITICAL: Failed to parse APP_BASE_URL '%s': %v", envConfig.AppBaseURL, constructErr)
				baseHandler.HandleError(httpResponseWriter, constructErr, utils.ServerError, "Internal configuration error generating URL.")
				return
			}
		} else {
			// If no base URL, create an empty URL struct; paths will be relative
			finalPublicURL = &url.URL{}
			applicationContext.Logger.Printf("WARN: Generating relative URL for QR code as APP_BASE_URL is not set.")
		}

		// Set the path component correctly
		finalPublicURL.Path = config.WebResponse // Just the path, e.g., "/response/"

		// Add the query parameter
		queryValues := finalPublicURL.Query()              // Get existing query params (if any)
		queryValues.Set(config.RSVPIDParam, rsvpRecord.ID) // Add or replace rsvp_id
		finalPublicURL.RawQuery = queryValues.Encode()     // Encode and set the query string

		publicURLString := finalPublicURL.String()
		// --- End Corrected URL Construction ---

		qrCodePNG, qrErr := qrcode.Encode(publicURLString, qrcode.Medium, 256)
		if qrErr != nil {
			applicationContext.Logger.Printf("ERROR: Failed to generate QR code for URL '%s': %v", publicURLString, qrErr)
			baseHandler.HandleError(httpResponseWriter, qrErr, utils.ServerError, "Failed to generate the QR code image.")
			return
		}

		qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCodePNG)

		viewData := RSVPShowViewData{
			RSVP:           rsvpRecord,
			Event:          eventRecord,
			QRCode:         qrCodeBase64,
			PublicURL:      publicURLString,
			URLForRSVPList: config.WebRSVPs,
			ParamEventID:   config.EventIDParam,
			ParamRSVPID:    config.RSVPIDParam,
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateRSVP, viewData)
	}
}
