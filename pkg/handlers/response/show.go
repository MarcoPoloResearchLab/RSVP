// Package response handles the public-facing RSVP response submission workflow.
package response

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/utils"
)

// ViewData is the data structure passed to the response.tmpl template (the RSVP form).
type ViewData struct {
	RSVP                 models.RSVP
	Event                models.Event
	URLForResponseSubmit string
	ParamRSVPID          string
	ParamMethodOverride  string
	ParamResponse        string
	ParamExtraGuests     string
	MaxGuestCount        int
}

// ThankYouViewData is the data structure passed to the thankyou.tmpl template.
type ThankYouViewData struct {
	Name                 string
	ThankYouMessage      string
	Code                 string
	URLForResponseChange string
	ParamRSVPID          string
}

// Handler processes requests for the public RSVP response page.
// It handles GET requests to display the form and PUT requests (via POST override) to submit the response.
// It requires a valid RSVP ID (code) in the query parameters.
// Assumes backend expects separate 'response' ('Yes'/'No') and 'extra_guests' parameters.
func Handler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameResponse, config.WebResponse)

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
		findRsvpError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findRsvpError != nil {
			if errors.Is(findRsvpError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.NotFoundError, "Invalid or expired RSVP identifier.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.DatabaseError, "Sorry, we encountered an error retrieving the RSVP details.")
			}
			return
		}

		var eventRecord models.Event
		eventError := eventRecord.LoadWithVenue(applicationContext.Database, rsvpRecord.EventID)
		if eventError != nil {
			applicationContext.Logger.Printf("ERROR: Could not find event %s associated with RSVP %s (using LoadWithVenue): %v", rsvpRecord.EventID, rsvpCode, eventError)
			errorType := utils.DatabaseError
			userMessage := "Sorry, we encountered an error loading event details."
			if errors.Is(eventError, gorm.ErrRecordNotFound) {
				errorType = utils.ServerError
				userMessage = "Could not load event details: The associated event seems to be missing."
			}
			baseHandler.HandleError(httpResponseWriter, eventError, errorType, userMessage)
			return
		}

		switch httpRequest.Method {
		case http.MethodGet:
			submitURL := utils.BuildRelativeURL(config.WebResponse, map[string]string{config.RSVPIDParam: rsvpCode})

			viewData := ViewData{
				RSVP:                 rsvpRecord,
				Event:                eventRecord,
				URLForResponseSubmit: submitURL,
				ParamRSVPID:          config.RSVPIDParam,
				ParamMethodOverride:  config.MethodOverrideParam,
				ParamResponse:        config.ResponseParam,
				ParamExtraGuests:     config.ExtraGuestsParam,
				MaxGuestCount:        config.MaxGuestCount,
			}
			baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateResponse, viewData)

		case http.MethodPut:
			if parseError := httpRequest.ParseForm(); parseError != nil {
				if !errors.Is(parseError, http.ErrNotMultipart) {
					baseHandler.HandleError(httpResponseWriter, parseError, utils.ValidationError, "Invalid form submission data.")
					return
				}
			}

			responseStatus := httpRequest.FormValue(config.ResponseParam)
			extraGuestsStr := httpRequest.FormValue(config.ExtraGuestsParam)
			var extraGuests int = 0

			if validationError := utils.ValidateRSVPResponseStatus(responseStatus); validationError != nil {
				baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
				return
			}

			if responseStatus == config.RSVPResponseYesPrefix {
				var parseErr error
				extraGuests, parseErr = strconv.Atoi(extraGuestsStr)
				if parseErr != nil {
					baseHandler.HandleError(httpResponseWriter, parseErr, utils.ValidationError, "Invalid value provided for extra guests.")
					return
				}
				if validationError := utils.ValidateExtraGuests(extraGuests); validationError != nil {
					baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
					return
				}
				rsvpRecord.Response = config.RSVPResponseYesPrefix
				rsvpRecord.ExtraGuests = extraGuests
			} else if responseStatus == config.RSVPResponseNo {
				rsvpRecord.Response = config.RSVPResponseNoCommaZero
				rsvpRecord.ExtraGuests = 0
			} else {
				baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Invalid response status submitted.")
				return
			}

			if saveError := rsvpRecord.Save(applicationContext.Database); saveError != nil {
				baseHandler.HandleError(httpResponseWriter, saveError, utils.DatabaseError, "Failed to save your RSVP response. Please try again.")
				return
			}

			redirectURL := utils.BuildRelativeURL(config.WebResponseThankYou, map[string]string{config.RSVPIDParam: rsvpCode})
			http.Redirect(httpResponseWriter, httpRequest, redirectURL, http.StatusSeeOther)

		default:
			baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet, http.MethodPut)
		}
	}
}

// ThankYouHandler handles GET requests for the public "Thank You" page displayed after RSVP submission.
// It requires a valid RSVP ID (code) in the query parameters.
func ThankYouHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameThankYou, config.WebResponseThankYou)

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
		findRsvpError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findRsvpError != nil {
			if errors.Is(findRsvpError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.NotFoundError, "Invalid or expired RSVP identifier.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}

		var thankYouMessageText string
		if rsvpRecord.Response == config.RSVPResponseYesPrefix {
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

		changeResponseURL := utils.BuildRelativeURL(config.WebResponse, map[string]string{config.RSVPIDParam: rsvpCode})

		viewData := ThankYouViewData{
			Name:                 rsvpRecord.Name,
			ThankYouMessage:      thankYouMessageText,
			Code:                 rsvpRecord.ID,
			URLForResponseChange: changeResponseURL,
			ParamRSVPID:          config.RSVPIDParam,
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateThankYou, viewData)
	}
}
