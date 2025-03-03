package handlers

import (
	"encoding/base64"
	"fmt"
	"github.com/temirov/RSVP/pkg/utils"
	"net/http"

	"github.com/skip2/go-qrcode"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// generateQRCode creates a base64-encoded PNG from the provided data.
func generateQRCode(dataString string, applicationContext *config.App) string {
	qrCodeBytes, errorValue := qrcode.Encode(dataString, qrcode.Medium, 256)
	if errorValue != nil {
		applicationContext.Logger.Fatal(errorValue)
	}
	return base64.StdEncoding.EncodeToString(qrCodeBytes)
}

// GenerateHandler creates a new RSVP record and returns a QR code.
func GenerateHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method == http.MethodPost {
			inviteeName := httpRequest.FormValue("name")
			newRSVP := models.RSVP{
				Name: inviteeName,
				Code: utils.Base36Encode6(),
			}
			errorValue := newRSVP.Create(applicationContext.Database)
			if errorValue != nil {
				applicationContext.Logger.Println("Error creating invite:", errorValue)
				http.Error(httpResponseWriter, "Could not create invite", http.StatusInternalServerError)
				return
			}
			protocolValue := "http"
			if httpRequest.TLS != nil {
				protocolValue = "https"
			}
			rsvpURL := fmt.Sprintf("%s://%s/rsvp?code=%s", protocolValue, httpRequest.Host, newRSVP.Code)
			qrBase64 := generateQRCode(rsvpURL, applicationContext)
			loggedUserData := getUserData(httpRequest, applicationContext)
			data := struct {
				Name           string
				QRCode         string
				RsvpURL        string
				LoggedUserData LoggedUserData
			}{
				Name:           newRSVP.Name,
				QRCode:         qrBase64,
				RsvpURL:        rsvpURL,
				LoggedUserData: *loggedUserData,
			}
			errorValue = applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "generate.html", data)
			if errorValue != nil {
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				applicationContext.Logger.Printf("failed to render template generate.html: %v", errorValue)
				return
			}
			return
		}
		http.Redirect(httpResponseWriter, httpRequest, config.WebRoot, http.StatusSeeOther)
	}
}
