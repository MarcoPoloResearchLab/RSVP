package rsvp

import (
	"encoding/base64"
	"fmt"
	"github.com/temirov/RSVP/pkg/handlers"
	"net/http"

	"github.com/skip2/go-qrcode"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// Show handles GET /rsvps/{code}/qr and displays the QR code for the RSVP link.
func Show(applicationContext *config.ApplicationContext, rsvpCode string) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if rsvpCode == "" || !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(responseWriter, "Invalid or missing RSVP code", http.StatusBadRequest)
			return
		}
		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode); findError != nil {
			applicationContext.Logger.Println("RSVP not found for QR generation:", rsvpCode, findError)
			http.Error(responseWriter, "RSVP not found", http.StatusNotFound)
			return
		}
		scheme := "http"
		if request.TLS != nil {
			scheme = "https"
		}
		rsvpURL := fmt.Sprintf("%s://%s%s/%s", scheme, request.Host, config.WebRSVPs, rsvpRecord.Code)
		qrCodeBytes, qrError := qrcode.Encode(rsvpURL, qrcode.Medium, 256)
		if qrError != nil {
			applicationContext.Logger.Println("QR code encoding failed:", qrError)
			http.Error(responseWriter, "Failed to generate QR code", http.StatusInternalServerError)
			return
		}
		qrBase64String := base64.StdEncoding.EncodeToString(qrCodeBytes)
		userSessionData := handlers.GetUserData(request, applicationContext)
		templateData := struct {
			Name        string
			QRCode      string
			RsvpURL     string
			UserPicture string
			UserName    string
		}{
			Name:        rsvpRecord.Name,
			QRCode:      qrBase64String,
			RsvpURL:     rsvpURL,
			UserPicture: userSessionData.UserPicture,
			UserName:    userSessionData.UserName,
		}
		if templateError := applicationContext.Templates.ExecuteTemplate(responseWriter, "generate.html", templateData); templateError != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render generate.html: %v", templateError)
		}
	})
}
