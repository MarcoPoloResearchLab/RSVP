package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/temirov/RSVP/utils"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/temirov/RSVP/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/skip2/go-qrcode"

	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
)

const (
	WebRoot      = "/"
	WebGenerate  = "/generate"
	WebRSVP      = "/rsvp"
	WebSubmit    = "/submit"
	WebResponses = "/responses"
	WebThankYou  = "/thankyou"
	HTTPPort     = 8080
	HTTPIP       = "0.0.0.0"
)

var (
	db        *gorm.DB
	templates = template.Must(template.ParseGlob("templates/*.html"))
)

// initDatabase sets up the SQLite DB with GORM and migrates our RSVP model.
func initDatabase() {
	var err error
	db, err = gorm.Open(sqlite.Open("rsvps.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// AutoMigrate will create or update the schema for our RSVP struct.
	if err := db.AutoMigrate(&models.RSVP{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
}

// generateQRCode creates a base64-encoded PNG QR image from the given string.
func generateQRCode(data string) string {
	qrBytes, err := qrcode.Encode(data, qrcode.Medium, 256)
	if err != nil {
		log.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(qrBytes)
}

// indexHandler displays a simple form to create a new invite.
func indexHandler(responseWriter http.ResponseWriter, request *http.Request) {
	errorExecute := templates.ExecuteTemplate(responseWriter, "index.html", nil)
	if errorExecute != nil {
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("failed to render template index.html: %v", errorExecute)
		return
	}
}

// generateHandler creates a new RSVP record with a 6-digit base36 code, then displays a QR code.
func generateHandler(responseWriter http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		inviteeName := r.FormValue("name")

		// Build a new RSVP with a random 6-digit code.
		rsvp := models.RSVP{
			Name: inviteeName,
			Code: utils.Base36Encode6(),
		}
		if err := rsvp.Create(db); err != nil {
			log.Println("Error creating invite:", err)
			http.Error(responseWriter, "Could not create invite", http.StatusInternalServerError)
			return
		}

		// Construct an RSVP URL using the code.
		rsvpURL := fmt.Sprintf("http://%s/rsvp?code=%s", r.Host, rsvp.Code)
		qrBase64 := generateQRCode(rsvpURL)

		data := struct {
			Name    string
			QRCode  string
			RsvpURL string
		}{
			Name:    rsvp.Name,
			QRCode:  qrBase64,
			RsvpURL: rsvpURL,
		}
		errorExecute := templates.ExecuteTemplate(responseWriter, "generate.html", data)
		if errorExecute != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("failed to render template index.html: %v", errorExecute)
			return
		}
		return
	}
	http.Redirect(responseWriter, r, "/", http.StatusSeeOther)
}

// rsvpHandler fetches the RSVP by code and displays the RSVP page.
func rsvpHandler(responseWriter http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(responseWriter, r, "/", http.StatusSeeOther)
		return
	}

	var rsvp models.RSVP
	if err := rsvp.FindByCode(db, code); err != nil {
		log.Println("Could not find invite for code:", code, err)
		http.Error(responseWriter, "Invite not found", http.StatusNotFound)
		return
	}

	displayName := rsvp.Name
	if displayName == "" {
		displayName = "Friend"
	}

	// Directly build the current answer from the DB values.
	currentAnswer := fmt.Sprintf("%s,%d", rsvp.Response, rsvp.ExtraGuests)

	data := struct {
		Name          string
		Code          string
		CurrentAnswer string
	}{
		Name:          displayName,
		Code:          code,
		CurrentAnswer: currentAnswer,
	}

	errorExecute := templates.ExecuteTemplate(responseWriter, "rsvp.html", data)
	if errorExecute != nil {
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("failed to render template index.html: %v", errorExecute)
		return
	}
}

// thankyouHandler fetches the RSVP by code and displays a thank-you message.
func thankyouHandler(responseWriter http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(responseWriter, "Code is required", http.StatusBadRequest)
		return
	}

	var rsvp models.RSVP
	if err := rsvp.FindByCode(db, code); err != nil {
		log.Println("Could not find invite for code:", code, err)
		http.Error(responseWriter, "Invite not found", http.StatusNotFound)
		return
	}

	// Build a message based on whether they said Yes or No.
	thankYouMessage := ""
	if rsvp.Response == "Yes" {
		thankYouMessage = fmt.Sprintf("We are looking forward to seeing you +%d!", rsvp.ExtraGuests)
	} else {
		thankYouMessage = "Sorry you couldn’t make it!"
	}

	// Provide the data to the template
	data := struct {
		Name            string
		Response        string
		ExtraGuests     int
		ThankYouMessage string
		Code            string
	}{
		Name:            rsvp.Name,
		Response:        rsvp.Response,
		ExtraGuests:     rsvp.ExtraGuests,
		ThankYouMessage: thankYouMessage,
		Code:            code,
	}

	errorExecute := templates.ExecuteTemplate(responseWriter, "thankyou.html", data)
	if errorExecute != nil {
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("failed to render template index.html: %v", errorExecute)
		return
	}
}

// submitHandler updates an RSVP's response and extra guests.
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		code := r.FormValue("code")
		responseValue := r.FormValue("response")

		parts := strings.Split(responseValue, ",")
		if len(parts) != 2 {
			parts = []string{"No", "0"}
		}
		rsvpResponse := parts[0]
		extraGuestsStr := parts[1]
		extraGuests, err := strconv.Atoi(extraGuestsStr)
		if err != nil {
			extraGuests = 0
		}

		var rsvp models.RSVP
		if err := rsvp.FindByCode(db, code); err != nil {
			log.Println("Could not find invite to update for code:", code, err)
			http.Error(w, "Invite not found", http.StatusNotFound)
			return
		}

		rsvp.Response = rsvpResponse
		rsvp.ExtraGuests = extraGuests
		if err := rsvp.Save(db); err != nil {
			log.Println("Could not save RSVP:", err)
			http.Error(w, "Could not update RSVP", http.StatusInternalServerError)
			return
		}

		// Redirect to /thankyou, passing the same code
		http.Redirect(w, r, "/thankyou?code="+code, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, WebRoot, http.StatusSeeOther)
}

// responsesHandler lists all RSVPs.
func responsesHandler(responseWriter http.ResponseWriter, r *http.Request) {
	var allRSVPs []models.RSVP
	if err := db.Find(&allRSVPs).Error; err != nil {
		log.Println("Error fetching RSVPs:", err)
		http.Error(responseWriter, "Could not retrieve RSVPs", http.StatusInternalServerError)
		return
	}

	for i := range allRSVPs {
		if allRSVPs[i].Response == "" {
			allRSVPs[i].Response = "Pending"
		}
	}

	errorExecute := templates.ExecuteTemplate(responseWriter, "responses.html", allRSVPs)
	if errorExecute != nil {
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("failed to render template index.html: %v", errorExecute)
		return
	}
}

func main() {
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET is not set")
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		log.Fatal("GOOGLE_CLIENT_ID is not set")
	}

	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_SECRET is not set")
	}

	googleOauth2Base := os.Getenv("GOOGLE_OAUTH2_BASE")
	if googleOauth2Base == "" {
		log.Fatal("GOOGLE_OAUTH2_BASE is not set")
	}

	session.NewSession([]byte(sessionSecret))
	authService, err := gauss.NewService(googleClientID, googleClientSecret, googleOauth2Base, WebRoot)
	if err != nil {
		log.Fatal("Failed to initialize auth service:", err)
	}

	authHandlers, err := gauss.NewHandlers(authService)
	if err != nil {
		log.Fatal("Failed to initialize handlers:", err)
	}

	initDatabase()

	// Set up HTTP handlers
	handler := http.NewServeMux()
	authHandlers.RegisterRoutes(handler)

	protectedIndexHandler := gauss.AuthMiddleware(utils.HTTPHandlerWrapper(indexHandler))
	protectedResponsesHandler := gauss.AuthMiddleware(utils.HTTPHandlerWrapper(responsesHandler))
	protectedGenerateHandler := gauss.AuthMiddleware(utils.HTTPHandlerWrapper(generateHandler))
	// Register the protected handlers using Handle instead of HandleFunc
	handler.Handle(WebRoot, protectedIndexHandler)
	handler.Handle(WebGenerate, protectedGenerateHandler)
	handler.Handle(WebResponses, protectedResponsesHandler)

	handler.HandleFunc(WebRSVP, rsvpHandler)
	handler.HandleFunc(WebSubmit, submitHandler)
	handler.HandleFunc(WebThankYou, thankyouHandler)

	httpSeverAddress := HTTPIP
	if httpSeverAddress == "127.0.0.1" {
		httpSeverAddress = "localhost"
	}

	// Start the HTTP server with graceful shutdown
	addr := fmt.Sprintf("%s:%d", httpSeverAddress, HTTPPort)
	httpServer := startHTTPServer(addr, handler)

	// Listen for interrupt signals to gracefully shut down the server
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	shutdownCtx, cancel := context.WithCancel(context.Background())

	// Shutdown triggered when the stop signal is received
	<-stop
	log.Println("Received shutdown signal, initiating graceful shutdown...")
	cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server Shutdown: %v", err)
	} else {
		log.Println("HTTP server gracefully stopped")
	}

}

func startHTTPServer(address string, handler http.Handler) *http.Server {
	httpServer := &http.Server{
		Addr:    address,
		Handler: handler,
	}

	go func() {
		log.Printf("Server starting on http://%s", address)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Error starting server: %v", err)
		}
	}()

	return httpServer
}
