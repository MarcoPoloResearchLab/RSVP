package event

//
//import (
//	"net/http"
//	"time"
//
//	"github.com/temirov/RSVP/models"
//	"github.com/temirov/RSVP/pkg/config"
//	"github.com/temirov/RSVP/pkg/handlers"
//)
//
//// EventIndexHandler handles GET and POST requests on "/".
//// GET: Loads all events for the logged‑in user and renders the dashboard.
//// POST: Processes the new event form, creates the event, and redirects back.
//func EventIndexHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
//	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
//		sessionData := handlers.GetUserData(httpRequest, applicationContext)
//		if sessionData.UserEmail == "" {
//			http.Redirect(httpResponseWriter, httpRequest, "/login", http.StatusSeeOther)
//			return
//		}
//
//		var currentUser models.User
//		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
//			// Instead of redirecting to login, upsert (create) the user record.
//			newUser, upsertError := models.UpsertUser(applicationContext.Database, sessionData.UserEmail, sessionData.UserName, sessionData.UserPicture)
//			if upsertError != nil {
//				applicationContext.Logger.Println("Failed to upsert user:", upsertError)
//				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
//				return
//			}
//			currentUser = *newUser
//		}
//
//		if httpRequest.Method == http.MethodGet {
//			var userEvents []models.Event
//			if queryError := applicationContext.Database.Where("user_id = ?", currentUser.ID).Find(&userEvents).Error; queryError != nil {
//				applicationContext.Logger.Println("Error retrieving events:", queryError)
//				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
//				return
//			}
//			templateData := struct {
//				UserPicture string
//				UserName    string
//				Events      []models.Event
//			}{
//				UserPicture: sessionData.UserPicture,
//				UserName:    sessionData.UserName,
//				Events:      userEvents,
//			}
//			if renderError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "index.html", templateData); renderError != nil {
//				applicationContext.Logger.Printf("Error rendering template: %v", renderError)
//				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
//			}
//		} else if httpRequest.Method == http.MethodPost {
//			eventTitle := httpRequest.FormValue("title")
//			eventDescription := httpRequest.FormValue("description")
//			eventStartTimeStr := httpRequest.FormValue("start_time")
//			eventEndTimeStr := httpRequest.FormValue("end_time")
//
//			if eventTitle == "" || eventStartTimeStr == "" || eventEndTimeStr == "" {
//				http.Error(httpResponseWriter, "Title, start time and end time are required", http.StatusBadRequest)
//				return
//			}
//
//			const timeLayout = "2006-01-02T15:04"
//			eventStartTime, startTimeError := time.Parse(timeLayout, eventStartTimeStr)
//			eventEndTime, endTimeError := time.Parse(timeLayout, eventEndTimeStr)
//			if startTimeError != nil || endTimeError != nil {
//				http.Error(httpResponseWriter, "Invalid date/time format", http.StatusBadRequest)
//				return
//			}
//
//			newEvent := models.Event{
//				Title:       eventTitle,
//				Description: eventDescription,
//				StartTime:   eventStartTime,
//				EndTime:     eventEndTime,
//				UserID:      currentUser.ID,
//			}
//			if creationError := newEvent.Create(applicationContext.Database); creationError != nil {
//				applicationContext.Logger.Println("Error creating event:", creationError)
//				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
//				return
//			}
//			http.Redirect(httpResponseWriter, httpRequest, "/", http.StatusSeeOther)
//		} else {
//			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
//		}
//	}
//}
