package rsvp

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/skip2/go-qrcode"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// VisualizationHandler handles GET /rsvps?id={id}&print=true to display a printable QR code.
func VisualizationHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(httpResponseWriter, "Missing RSVP ID", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpIdentifier)
		if findError != nil {
			baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		var eventRecord models.Event
		findEventError := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID)
		if findEventError != nil {
			baseHandler.HandleError(httpResponseWriter, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		if eventRecord.UserID != currentUser.ID {
			http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
			return
		}

		rsvpURLObject := url.URL{
			Scheme: "http",
			Host:   httpRequest.Host,
			Path:   "/rsvp",
		}
		if httpRequest.TLS != nil {
			rsvpURLObject.Scheme = "https"
		}
		queryParams := rsvpURLObject.Query()
		queryParams.Set("code", rsvpRecord.ID)
		rsvpURLObject.RawQuery = queryParams.Encode()
		finalRSVPURL := rsvpURLObject.String()

		qrCodeBytes, qrCodeError := qrcode.Encode(finalRSVPURL, qrcode.Medium, 256)
		if qrCodeError != nil {
			baseHandler.HandleError(httpResponseWriter, qrCodeError, utils.ServerError, "Failed to generate QR code")
			return
		}
		qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCodeBytes)

		templateData := struct {
			UserName     string
			UserPicture  string
			RSVP         models.RSVP
			Event        models.Event
			QRCodeBase64 string
			RSVPUrl      string
		}{
			UserName:     sessionData.UserName,
			UserPicture:  sessionData.UserPicture,
			RSVP:         rsvpRecord,
			Event:        eventRecord,
			QRCodeBase64: qrCodeBase64,
			RSVPUrl:      finalRSVPURL,
		}

		baseHandler.RenderTemplate(httpResponseWriter, config.TemplateQR, templateData)
	}
}
