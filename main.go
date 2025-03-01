package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
)

var databaseSQL *sql.DB
var templates = template.Must(template.ParseGlob("templates/*.html"))

func initDatabase() {
	var err error
	databaseSQL, err = sql.Open("sqlite3", "rsvps.db")
	if err != nil {
		log.Fatal(err)
	}

	// Add a "base36" column to store unique IDs for those without emails.
	// We'll also store name, email, response, and extra_guests.
	createTableStatement := `
        CREATE TABLE IF NOT EXISTS rsvps (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT,
            email TEXT UNIQUE,
            base36 TEXT UNIQUE,
            response TEXT,
            extra_guests INTEGER DEFAULT 0
        );
    `

	_, err = databaseSQL.Exec(createTableStatement)
	if err != nil {
		log.Fatal(err)
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

// base36Encode converts an integer to a base36 string.
// If you want a shorter random approach, we can do a direct random-based ID.
// But for a guaranteed unique, we often tie to a DB row.
func base36Encode(num int) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	if num == 0 {
		return "0"
	}
	result := ""
	for num > 0 {
		remainder := num % 36
		num = num / 36
		result = string(charset[remainder]) + result
	}
	return result
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

// generateInvite inserts a row with name/email (if provided), then updates base36 if needed.
func generateInvite(inviteeName, inviteeEmail string) (string, error) {
	// If email is provided, use that as the unique key. If not, we will generate a base36.

	// Insert row ignoring duplicates. We'll handle the case if no email is provided by inserting with placeholders.
	insertStmt := `INSERT INTO rsvps (name, email) VALUES (?, ?)`
	res, err := databaseSQL.Exec(insertStmt, inviteeName, inviteeEmail)
	if err != nil {
		// possibly a constraint error if email is duplicate, etc.
		return "", err
	}

	// If email is empty, we need to set a base36 ID.
	if inviteeEmail == "" {
		// We'll get the last inserted row id.
		rowID, err := res.LastInsertId()
		if err != nil {
			return "", err
		}
		base36ID := base36Encode(int(rowID))

		// Update the row with the base36 ID
		updateStmt := `UPDATE rsvps SET base36 = ? WHERE id = ?`
		_, err = databaseSQL.Exec(updateStmt, base36ID, rowID)
		if err != nil {
			return "", err
		}
		return base36ID, nil
	}

	// If we had an email, we assume it was successful.
	return inviteeEmail, nil
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		inviteeName := r.FormValue("name")
		inviteeEmail := r.FormValue("email")

		identifier, err := generateInvite(inviteeName, inviteeEmail)
		if err != nil {
			log.Println("Error creating invite:", err)
			http.Error(w, "Could not create invite", http.StatusInternalServerError)
			return
		}

		// Construct a unique RSVP URL. We'll use 'identifier' for the query param.
		// This might be an email or a base36.
		rsvpURL := fmt.Sprintf("http://%s/rsvp?identifier=%s", r.Host, identifier)
		qrBase64 := generateQRCode(rsvpURL)

		data := struct {
			Name    string
			QRCode  string
			RsvpURL string
		}{
			Name:    inviteeName,
			QRCode:  qrBase64,
			RsvpURL: rsvpURL,
		}
		templates.ExecuteTemplate(w, "generate.html", data)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// rsvpHandler fetches the invite record based on either email or base36.
func rsvpHandler(w http.ResponseWriter, r *http.Request) {
	identifier := r.URL.Query().Get("identifier")
	if identifier == "" {
		// If no identifier is provided, redirect to home.
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Try to see if this is an email or a base36.
	// We'll do a single SELECT that checks both columns.
	row := databaseSQL.QueryRow(`
        SELECT name, email, base36 FROM rsvps
        WHERE email = ? OR base36 = ?
    `, identifier, identifier)

	var inviteeName, inviteeEmail, inviteeBase36 string
	err := row.Scan(&inviteeName, &inviteeEmail, &inviteeBase36)
	if err != nil {
		log.Println("Could not find invite for:", identifier, err)
		// Optionally show an error page.
		http.Error(w, "Invite not found", http.StatusNotFound)
		return
	}

	// We'll display the name if present, else fallback.
	if inviteeName == "" {
		inviteeName = "Friend" // or any placeholder
	}

	// We'll pass these to the template.
	data := struct {
		Name       string
		Identifier string
	}{
		Name:       inviteeName,
		Identifier: identifier,
	}

	templates.ExecuteTemplate(w, "rsvp.html", data)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		identifier := r.FormValue("identifier")
		formValue := r.FormValue("response")

		parts := strings.Split(formValue, ",")
		if len(parts) != 2 {
			parts = []string{"No", "0"}
		}
		rsvpResponse := parts[0]
		extraGuestsStr := parts[1]
		extraGuests, err := strconv.Atoi(extraGuestsStr)
		if err != nil {
			extraGuests = 0
		}

		// We'll update based on either email or base36.
		updateStmt := `
            UPDATE rsvps
            SET response = ?, extra_guests = ?
            WHERE email = ? OR base36 = ?
        `
		_, err = databaseSQL.Exec(updateStmt, rsvpResponse, extraGuests, identifier, identifier)
		if err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/responses", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func responsesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := databaseSQL.Query(`
        SELECT name, email, base36, response, extra_guests
        FROM rsvps
    `)
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	type rsvpRow struct {
		Name        string
		Email       string
		Base36      string
		Response    string
		ExtraGuests int
	}

	var allRSVPs []rsvpRow
	for rows.Next() {
		var nameVal, emailVal, base36Val, respVal string
		var extraVal int
		err := rows.Scan(&nameVal, &emailVal, &base36Val, &respVal, &extraVal)
		if err != nil {
			log.Println(err)
			continue
		}
		if respVal == "" {
			respVal = "Pending"
		}
		allRSVPs = append(allRSVPs, rsvpRow{
			Name:        nameVal,
			Email:       emailVal,
			Base36:      base36Val,
			Response:    respVal,
			ExtraGuests: extraVal,
		})
	}

	templates.ExecuteTemplate(w, "responses.html", allRSVPs)
}

func main() {
	// Seed rand if needed for some random approach, else not used in this example.
	rand.Seed(time.Now().UnixNano())

	initDatabase()
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/generate", generateHandler)
	http.HandleFunc("/rsvp", rsvpHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/responses", responsesHandler)

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
