package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
)

// IndexHandler renders the events page as the index page.
func IndexHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		loggedUserData := GetUserData(httpRequest, applicationContext)
		if templateError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.TemplateEvents, loggedUserData); templateError != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render %s: %v", config.TemplateEvents, templateError)
			return
		}
	}
}
