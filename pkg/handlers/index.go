package handlers

import (
	"net/http"

	"github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
)

// LoggedUserData holds user session details.
type LoggedUserData struct {
	UserPicture string
	UserName    string
}

// getUserData retrieves the logged user data from the session.
func getUserData(httpRequest *http.Request, applicationContext *config.App) *LoggedUserData {
	sessionInstance, _ := session.Store().Get(httpRequest, constants.SessionName)
	userPicture, _ := sessionInstance.Values["user_picture"].(string)
	userName, _ := sessionInstance.Values["user_name"].(string)
	return &LoggedUserData{
		UserPicture: userPicture,
		UserName:    userName,
	}
}

// IndexHandler renders the index page.
func IndexHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		loggedUserData := getUserData(httpRequest, applicationContext)
		errorValue := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "index.html", loggedUserData)
		if errorValue != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("failed to render template index.html: %v", errorValue)
			return
		}
	}
}
