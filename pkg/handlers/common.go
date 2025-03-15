package handlers

import (
	"net/http"
	"regexp"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
)

// LoggedUserData holds user session details for RSVP and event templates.
type LoggedUserData struct {
	UserPicture string
	UserName    string
	UserEmail   string
}

// GetUserData extracts the current user data from the session.
func GetUserData(httpRequest *http.Request, applicationContext *config.ApplicationContext) *LoggedUserData {
	sessionInstance, sessionError := session.Store().Get(httpRequest, gconstants.SessionName)
	if sessionError != nil {
		applicationContext.Logger.Println("Error retrieving session:", sessionError)
		return &LoggedUserData{}
	}
	sessionUserPicture, pictureOk := sessionInstance.Values[gconstants.SessionKeyUserPicture].(string)
	if !pictureOk {
		applicationContext.Logger.Printf("Error retrieving session user Picture %s", sessionUserPicture)
		sessionUserPicture = ""
	}
	sessionUserName, nameOk := sessionInstance.Values[gconstants.SessionKeyUserName].(string)
	if !nameOk {
		applicationContext.Logger.Printf("Error retrieving session user name %s", sessionUserName)
		sessionUserName = ""
	}
	sessionUserEmail, emailOk := sessionInstance.Values[gconstants.SessionKeyUserEmail].(string)
	if !emailOk {
		applicationContext.Logger.Printf("Error retrieving session user email %s", sessionUserEmail)
		sessionUserEmail = ""
	}
	return &LoggedUserData{
		UserPicture: sessionUserPicture,
		UserName:    sessionUserName,
		UserEmail:   sessionUserEmail,
	}
}

// ValidateRSVPCode ensures the RSVP code is alphanumeric and exactly 6 characters long.
func ValidateRSVPCode(rsvpCode string) bool {
	var validCodePattern = regexp.MustCompile(`^[0-9a-z]{6}$`)
	return validCodePattern.MatchString(rsvpCode)
}
