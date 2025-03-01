package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
)

// databaseSQL is the global database handle.
var databaseSQL *sql.DB

// templates is the global template cache. It parses all .html files in the templates folder.
var templates = template.Must(template.ParseGlob("templates/*.html"))

// initDatabase initializes the SQLite database and creates/updates the RSVPs table if it doesn't exist.
func initDatabase() {
	var databaseError error
	databaseSQL, databaseError = sql.Open("sqlite3", "rsvps.db")
	if databaseError != nil {
		log.Fatal(databaseError)
	}

	// Create a table for RSVP data.
	// We include a new column extra_guests to track how many extra attendees (0 if alone).
	createTableStatement := `
        CREATE TABLE IF NOT EXISTS rsvps (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT NOT NULL UNIQUE,
            response TEXT,
            extra_guests INTEGER DEFAULT 0
        );
    `
	_, databaseError = databaseSQL.Exec(createTableStatement)
	if databaseError != nil {
		log.Fatal(databaseError)
	}
}

// generateQRCode creates a base64-encoded QR image from the given text data.
func generateQRCode(data string) string {
	qrBytes, generateError := qrcode.Encode(data, qrcode.Medium, 256)
	if generateError != nil {
		log.Fatal(generateError)
	}
	return base64.StdEncoding.EncodeToString(qrBytes)
}

// indexHandler displays a simple form to collect invitee details.
func indexHandler(responseWriter http.ResponseWriter, request *http.Request) {
	// Render index.html
	templates.ExecuteTemplate(responseWriter, "index.html", nil)
}

// generateHandler processes the form submission and generates a QR code.
func generateHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		inviteeName := request.FormValue("name")
		inviteeEmail := request.FormValue("email")

		// Construct a unique RSVP URL
		rsvpURL := fmt.Sprintf("http://%s/rsvp?email=%s", request.Host, inviteeEmail)
		qrBase64 := generateQRCode(rsvpURL)

		// Insert invitee into the database if not present
		_, databaseError := databaseSQL.Exec(
			"INSERT OR IGNORE INTO rsvps (name, email) VALUES (?, ?)",
			inviteeName, inviteeEmail,
		)
		if databaseError != nil {
			log.Fatal(databaseError)
		}

		// Render generate.html, passing data to display the QR code and link
		data := struct {
			Name    string
			QRCode  string
			RsvpURL string
		}{
			Name:    inviteeName,
			QRCode:  qrBase64,
			RsvpURL: rsvpURL,
		}
		templates.ExecuteTemplate(responseWriter, "generate.html", data)
	} else {
		// If someone tries a GET on /generate, just redirect to the home page
		http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
	}
}

// rsvpHandler shows the RSVP form so an invitee can choose Yes or No, and number of guests.
func rsvpHandler(responseWriter http.ResponseWriter, request *http.Request) {
	inviteeEmail := request.URL.Query().Get("email")

	// Query the database for the invitee’s name
	row := databaseSQL.QueryRow("SELECT name FROM rsvps WHERE email = ?", inviteeEmail)

	var inviteeName string
	err := row.Scan(&inviteeName)
	if err != nil {
		// If no record or an error occurs, log it and set a fallback
		log.Println("Error retrieving name:", err)
		inviteeName = "Unknown"
	}

	// Pass the name and email to the template
	data := struct {
		Name  string
		Email string
	}{
		Name:  inviteeName,
		Email: inviteeEmail,
	}

	templates.ExecuteTemplate(responseWriter, "rsvp.html", data)
}

// submitHandler updates the invitee's response in the database.
func submitHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		inviteeEmail := request.FormValue("email")
		formValue := request.FormValue("response")
		// We'll receive something like "Yes,0" or "Yes,2" or "No,0"

		parts := strings.Split(formValue, ",")
		if len(parts) != 2 {
			// If it's not in the format we expect, default to No
			parts = []string{"No", "0"}
		}
		rsvpResponse := parts[0]
		extraGuestsStr := parts[1]
		extraGuests, err := strconv.Atoi(extraGuestsStr)
		if err != nil {
			extraGuests = 0
		}

		_, databaseError := databaseSQL.Exec(
			"UPDATE rsvps SET response = ?, extra_guests = ? WHERE email = ?",
			rsvpResponse, extraGuests, inviteeEmail,
		)
		if databaseError != nil {
			log.Fatal(databaseError)
		}

		// Redirect to /responses to see the updated list
		http.Redirect(responseWriter, request, "/responses", http.StatusSeeOther)
	} else {
		http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
	}
}

// responsesHandler retrieves and displays all invitee responses.
func responsesHandler(responseWriter http.ResponseWriter, request *http.Request) {
	rows, queryError := databaseSQL.Query("SELECT name, email, response, extra_guests FROM rsvps")
	if queryError != nil {
		log.Fatal(queryError)
	}
	defer rows.Close()

	// A struct to represent the RSVP row
	type rsvpRow struct {
		Name        string
		Email       string
		Response    string
		ExtraGuests int
	}

	var allRSVPs []rsvpRow
	for rows.Next() {
		var nameValue, emailValue, responseValue string
		var extraGuestsValue int
		scanError := rows.Scan(&nameValue, &emailValue, &responseValue, &extraGuestsValue)
		if scanError != nil {
			log.Println(scanError)
			continue
		}
		if responseValue == "" {
			responseValue = "Pending"
		}
		allRSVPs = append(allRSVPs, rsvpRow{
			Name:        nameValue,
			Email:       emailValue,
			Response:    responseValue,
			ExtraGuests: extraGuestsValue,
		})
	}

	templates.ExecuteTemplate(responseWriter, "responses.html", allRSVPs)
}

func main() {
	// Initialize the database
	initDatabase()

	// Define our routes
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/generate", generateHandler)
	http.HandleFunc("/rsvp", rsvpHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/responses", responsesHandler)

	// Start the server
	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
