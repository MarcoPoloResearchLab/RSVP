package rsvp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// UpdateHandler handles PUT/POST requests to update an existing RSVP.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for RSVPs
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(w, r, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		// Get RSVP ID parameter
		rsvpID := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpID == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		// Load the existing RSVP from the database
		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpID); findError != nil {
			baseHandler.HandleError(w, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(w, r)

		// Find the current user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			baseHandler.HandleError(w, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		// Define a function to find the owner ID of an event
		findEventOwnerID := func(eventID string) (string, error) {
			var event models.Event
			if err := event.FindByID(applicationContext.Database, eventID); err != nil {
				return "", err
			}
			return event.UserID, nil
		}

		// Verify that the current user owns the event that this RSVP belongs to
		if !baseHandler.VerifyResourceOwnership(w, rsvpRecord.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Get updated RSVP details from the form
		params := baseHandler.GetParams(r, "name", "response", "extra_guests")

		// Update RSVP fields if provided
		if params["name"] != "" {
			// Validate name
			if nameError := utils.ValidateRSVPName(params["name"]); nameError != nil {
				baseHandler.HandleError(w, nameError, utils.ValidationError, nameError.Error())
				return
			}
			rsvpRecord.Name = params["name"]
		}

		if params["response"] != "" {
			// Validate response
			if responseError := utils.ValidateRSVPResponse(params["response"]); responseError != nil {
				baseHandler.HandleError(w, responseError, utils.ValidationError, responseError.Error())
				return
			}
			rsvpRecord.Response = params["response"]

			// If response is "Yes,N", update extra_guests accordingly
			if params["response"] != "No" && len(params["response"]) > 4 {
				parts := strings.Split(params["response"], ",")
				if len(parts) == 2 {
					guestCount, parseError := strconv.Atoi(parts[1])
					if parseError == nil {
						rsvpRecord.ExtraGuests = guestCount
					}
				}
			} else if params["response"] == "No" {
				// If response is "No", set extra_guests to 0
				rsvpRecord.ExtraGuests = 0
			}
		} else if params["extra_guests"] != "" {
			// If extra_guests is provided separately
			newExtraGuests, parseError := strconv.Atoi(params["extra_guests"])
			if parseError != nil {
				baseHandler.HandleError(w, parseError, utils.ValidationError, "Invalid extra guests value")
				return
			}

			// Validate guest count
			if newExtraGuests < 0 || newExtraGuests > utils.MaxGuestCount {
				baseHandler.HandleError(w, errors.New("invalid guest count"), utils.ValidationError,
					"Guest count must be between 0 and "+strconv.Itoa(utils.MaxGuestCount))
				return
			}

			rsvpRecord.ExtraGuests = newExtraGuests
		}

		// Save the updated RSVP
		if saveError := rsvpRecord.Save(applicationContext.Database); saveError != nil {
			baseHandler.HandleError(w, saveError, utils.DatabaseError, "Failed to update RSVP")
			return
		}

		// Check if event_id was provided (for admin editing flow)
		eventID := baseHandler.GetParam(r, config.EventIDParam)
		if eventID != "" {
			// Redirect back to the RSVP list page for this event
			baseHandler.RedirectWithParams(w, r, map[string]string{
				config.EventIDParam: eventID,
			})
		} else {
			// Redirect back to the RSVP detail page (for invitee flow)
			baseHandler.RedirectWithParams(w, r, map[string]string{
				config.RSVPIDParam: rsvpID,
			})
		}
	}
}
