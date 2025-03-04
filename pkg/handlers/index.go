package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
)

// IndexHandler renders the index page.
func IndexHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		loggedUserData := GetUserData(httpRequest, applicationContext)
		if templateError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.TemplateIndex, loggedUserData); templateError != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render %s: %v", config.TemplateIndex, templateError)
			return
		}
	}
}
