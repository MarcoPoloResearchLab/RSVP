package handlers

import (
	"net/http"
	"regexp"

	"github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
)

// LoggedUserData holds user session details for RSVP templates.
type LoggedUserData struct {
	UserPicture string
	UserName    string
}

// GetUserData extracts the current user data from the session.
func GetUserData(httpRequest *http.Request, applicationContext *config.App) *LoggedUserData {
	sessionInstance, sessionError := session.Store().Get(httpRequest, constants.SessionName)
	if sessionError != nil {
		applicationContext.Logger.Println("Error retrieving session:", sessionError)
		return &LoggedUserData{}
	}
	sessionUserPicture, pictureOk := sessionInstance.Values["user_picture"].(string)
	if !pictureOk {
		applicationContext.Logger.Printf("Error retrieving session user Picture %s", sessionUserPicture)
		sessionUserPicture = ""
	}
	sessionUserName, nameOk := sessionInstance.Values["user_name"].(string)
	if !nameOk {
		applicationContext.Logger.Printf("Error retrieving session user name %s", sessionUserName)
		sessionUserName = ""
	}
	return &LoggedUserData{
		UserPicture: sessionUserPicture,
		UserName:    sessionUserName,
	}
}

// ValidateRSVPCode ensures the RSVP code is alphanumeric and exactly 6 characters long.
func ValidateRSVPCode(rsvpCode string) bool {
	var validCodePattern = regexp.MustCompile(`^[0-9a-z]{6}$`)
	return validCodePattern.MatchString(rsvpCode)
}
