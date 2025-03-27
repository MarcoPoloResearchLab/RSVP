// File: pkg/handlers/response/show.go
package response

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ResponseViewData is the data structure passed to the response.tmpl template.
type ResponseViewData struct {
	RSVP  models.RSVP
	Event models.Event
	// Config values
	URLForResponseSubmit string
	ParamRSVPID          string // Added
	ParamMethodOverride  string
	ParamResponse        string
}

// ThankYouViewData is the data structure passed to the thankyou.tmpl template.
type ThankYouViewData struct {
	Name            string
	ThankYouMessage string
	Code            string
	// Config values
	URLForResponseChange string
	ParamRSVPID          string // Added
}

// Handler handles GET requests to show the RSVP form and PUT requests to submit it.
func Handler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "Response", config.WebResponse)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		rsvpCode := httpRequest.URL.Query().Get(config.RSVPIDParam)
		if rsvpCode == "" {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "RSVP identifier is missing from the URL.")
			return
		}
		if !handlers.ValidateRSVPCode(rsvpCode) {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Invalid RSVP identifier format.")
			return
		}

		var rsvpRecord models.RSVP
		findRsvpErr := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findRsvpErr != nil {
			if errors.Is(findRsvpErr, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.NotFoundError, "Invalid or expired RSVP identifier.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.DatabaseError, "Sorry, we encountered an error retrieving the RSVP details.")
			}
			return
		}

		var eventRecord models.Event
		eventErr := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID)
		if eventErr != nil {
			applicationContext.Logger.Printf("ERROR: Could not find event %s associated with RSVP %s: %v", rsvpRecord.EventID, rsvpCode, eventErr)
			if errors.Is(eventErr, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, eventErr, utils.ServerError, "Could not load event details: The associated event seems to be missing.")
			} else {
				baseHandler.HandleError(httpResponseWriter, eventErr, utils.DatabaseError, "Sorry, we encountered an error loading event details.")
			}
			return
		}

		submitURL := fmt.Sprintf("%s?%s=%s", config.WebResponse, config.RSVPIDParam, rsvpCode)

		// Prepare view data including config values
		viewData := ResponseViewData{
			RSVP:                 rsvpRecord,
			Event:                eventRecord,
			URLForResponseSubmit: submitURL,
			ParamRSVPID:          config.RSVPIDParam,         // Populate
			ParamMethodOverride:  config.MethodOverrideParam, // Populate
			ParamResponse:        config.ResponseParam,       // Populate
		}

		switch httpRequest.Method {
		case http.MethodGet:
			baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateResponse, viewData)

		case http.MethodPut:
			if err := httpRequest.ParseForm(); err != nil && !errors.Is(err, http.ErrNotMultipart) {
				baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, "Invalid form submission data.")
				return
			}

			userResponseValue := httpRequest.FormValue(config.ResponseParam) // Use constant from config
			if userResponseValue == "" {
				baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Please select a response option (Yes/No).")
				return
			}
			if validationErr := utils.ValidateRSVPResponse(userResponseValue); validationErr != nil {
				baseHandler.HandleError(httpResponseWriter, validationErr, utils.ValidationError, validationErr.Error())
				return
			}

			rsvpRecord.Response = userResponseValue
			if strings.HasPrefix(userResponseValue, "Yes") {
				parts := strings.Split(userResponseValue, ",")
				if len(parts) == 2 {
					guestCount, convErr := strconv.Atoi(parts[1])
					if convErr == nil {
						rsvpRecord.ExtraGuests = guestCount
					} else {
						applicationContext.Logger.Printf("WARN: Could not parse guest count from valid response '%s' for RSVP %s", userResponseValue, rsvpCode)
						rsvpRecord.ExtraGuests = 0
					}
				} else {
					applicationContext.Logger.Printf("WARN: Invalid 'Yes' response format '%s' after validation for RSVP %s", userResponseValue, rsvpCode)
					rsvpRecord.ExtraGuests = 0
				}
			} else {
				rsvpRecord.ExtraGuests = 0
				rsvpRecord.Response = "No,0"
			}

			if saveErr := rsvpRecord.Save(applicationContext.Database); saveErr != nil {
				baseHandler.HandleError(httpResponseWriter, saveErr, utils.DatabaseError, "Failed to save your RSVP response. Please try again.")
				return
			}

			redirectURL := fmt.Sprintf("%s?%s=%s", config.WebResponseThankYou, config.RSVPIDParam, rsvpCode)
			http.Redirect(httpResponseWriter, httpRequest, redirectURL, http.StatusSeeOther)

		default:
			baseHandler.ApplicationContext.Logger.Printf("Method Not Allowed for %s: Received %s", config.WebResponse, httpRequest.Method)
			http.Error(httpResponseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}
}

// ThankYouHandler handles GET requests for the public thank you page.
func ThankYouHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "ThankYou", config.WebResponseThankYou)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		rsvpCode := httpRequest.URL.Query().Get(config.RSVPIDParam)
		if rsvpCode == "" {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "RSVP identifier is missing.")
			return
		}
		if !handlers.ValidateRSVPCode(rsvpCode) {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Invalid RSVP identifier format.")
			return
		}

		var rsvpRecord models.RSVP
		findRsvpErr := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findRsvpErr != nil {
			if errors.Is(findRsvpErr, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.NotFoundError, "Invalid or expired RSVP identifier.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpErr, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}

		var thankYouMessageText string
		if strings.HasPrefix(rsvpRecord.Response, "Yes") {
			guests := rsvpRecord.ExtraGuests
			if guests == 0 {
				thankYouMessageText = "Your response is confirmed. We look forward to seeing you!"
			} else if guests == 1 {
				thankYouMessageText = "Your response is confirmed. We look forward to seeing you and your guest!"
			} else {
				thankYouMessageText = fmt.Sprintf("Your response is confirmed. We look forward to seeing you and your %d guests!", guests)
			}
		} else {
			thankYouMessageText = "Thank you for letting us know you can't make it."
		}

		changeURL := fmt.Sprintf("%s?%s=%s", config.WebResponse, config.RSVPIDParam, rsvpCode)

		// Prepare view data including config values
		viewData := ThankYouViewData{
			Name:                 rsvpRecord.Name,
			ThankYouMessage:      thankYouMessageText,
			Code:                 rsvpRecord.ID,
			URLForResponseChange: changeURL,
			ParamRSVPID:          config.RSVPIDParam, // Populate
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateThankYou, viewData)
	}
}
