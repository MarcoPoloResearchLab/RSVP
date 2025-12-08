package rsvp

import (
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
)

// ShowViewData holds data for the rsvp.tmpl (QR code display) view.
type ShowViewData struct {
	RSVP           models.RSVP
	Event          models.Event
	QRCode         string
	PublicURL      string
	URLForRSVPList string
	ParamEventID   string
	ParamRSVPID    string
}

// ShowHandler handles GET requests to display the QR code page for a specific RSVP.
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameRSVPQR, config.WebRSVPQR)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

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
		findRsvpError := applicationContext.Database.First(&rsvpRecord, "id = ?", rsvpID).Error
		if findRsvpError != nil {
			if errors.Is(findRsvpError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.NotFoundError, "The specified RSVP was not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findRsvpError, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}

		var eventRecord models.Event
		eventFindError := applicationContext.Database.First(&eventRecord, "id = ?", rsvpRecord.EventID).Error
		if eventFindError != nil {
			applicationContext.Logger.Printf("ERROR: Could not find parent event %s for RSVP %s during QR code generation request to %s", rsvpRecord.EventID, rsvpID, httpRequest.URL.Path)
			if errors.Is(eventFindError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Could not find the parent event for this RSVP.")
			} else {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.DatabaseError, "Error retrieving parent event details.")
			}
			return
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, eventRecord.UserID, currentUser.ID) {
			return
		}

		publicURLString, urlBuildError := utils.BuildPublicURL(
			applicationContext.AppBaseURL,
			config.WebResponse,
			map[string]string{config.RSVPIDParam: rsvpRecord.ID},
		)
		if urlBuildError != nil {
			applicationContext.Logger.Printf("CRITICAL: Failed to build public URL: %v", urlBuildError)
			baseHandler.HandleError(httpResponseWriter, urlBuildError, utils.ServerError, "Internal configuration error generating QR code URL.")
			return
		}

		qrCodePNG, qrError := qrcode.Encode(publicURLString, qrcode.Medium, 256)
		if qrError != nil {
			applicationContext.Logger.Printf("ERROR: Failed to generate QR code for URL '%s': %v", publicURLString, qrError)
			baseHandler.HandleError(httpResponseWriter, qrError, utils.ServerError, "Failed to generate the QR code image.")
			return
		}
		qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCodePNG)

		rsvpListURL := utils.BuildRelativeURL(config.WebRSVPs, map[string]string{config.EventIDParam: eventRecord.ID})

		viewData := ShowViewData{
			RSVP:           rsvpRecord,
			Event:          eventRecord,
			QRCode:         qrCodeBase64,
			PublicURL:      publicURLString,
			URLForRSVPList: rsvpListURL,
			ParamEventID:   config.EventIDParam,
			ParamRSVPID:    config.RSVPIDParam,
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateRSVP, viewData)
	}
}
