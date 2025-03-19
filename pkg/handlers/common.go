package handlers

import (
	"net/http"
	"regexp"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
)

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

	sessionUserPicture, pictureOK := sessionInstance.Values[gconstants.SessionKeyUserPicture].(string)
	if !pictureOK {
		applicationContext.Logger.Printf("Error retrieving session user Picture %s", sessionUserPicture)
		sessionUserPicture = ""
	}
	sessionUserName, nameOK := sessionInstance.Values[gconstants.SessionKeyUserName].(string)
	if !nameOK {
		applicationContext.Logger.Printf("Error retrieving session user name %s", sessionUserName)
		sessionUserName = ""
	}
	sessionUserEmail, emailOK := sessionInstance.Values[gconstants.SessionKeyUserEmail].(string)
	if !emailOK {
		applicationContext.Logger.Printf("Error retrieving session user email %s", sessionUserEmail)
		sessionUserEmail = ""
	}

	return &LoggedUserData{
		UserPicture: sessionUserPicture,
		UserName:    sessionUserName,
		UserEmail:   sessionUserEmail,
	}
}

// ValidateRSVPCode ensures the RSVP code is alphanumeric.
func ValidateRSVPCode(rsvpCode string) bool {
	var validCodePattern = regexp.MustCompile(`^[0-9a-zA-Z]{1,8}$`)
	return validCodePattern.MatchString(rsvpCode)
}
