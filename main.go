package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/skip2/go-qrcode"
)

// RSVP represents an invitation record with a base36 code, name, response, etc.
type RSVP struct {
	gorm.Model
	Name        string `gorm:"column:name"`
	Code        string `gorm:"column:code;uniqueIndex"`
	Response    string `gorm:"column:response"`
	ExtraGuests int    `gorm:"column:extra_guests;default:0"`
}

// FindByCode loads a single RSVP by its Code.
func (rsvp *RSVP) FindByCode(db *gorm.DB, code string) error {
	return db.Where("code = ?", code).First(rsvp).Error
}

// Create inserts a new RSVP into the database.
func (rsvp *RSVP) Create(db *gorm.DB) error {
	return db.Create(rsvp).Error
}

// Save updates an existing RSVP in the database.
func (rsvp *RSVP) Save(db *gorm.DB) error {
	return db.Save(rsvp).Error
}

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
	if err := db.AutoMigrate(&RSVP{}); err != nil {
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

// base36Encode6 returns a random 6-digit base36 string.
// This does NOT rely on incremental IDs; it just generates 6 random base36 chars.
func base36Encode6() string {
	const length = 6
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"

	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = chars[rand.Intn(len(chars))]
	}
	return string(out)
}

// indexHandler displays a simple form to create a new invite.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

// generateHandler creates a new RSVP record with a 6-digit base36 code, then displays a QR code.
func generateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		inviteeName := r.FormValue("name")

		// Build a new RSVP with a random 6-digit code.
		rsvp := RSVP{
			Name: inviteeName,
			Code: base36Encode6(),
		}
		if err := rsvp.Create(db); err != nil {
			log.Println("Error creating invite:", err)
			http.Error(w, "Could not create invite", http.StatusInternalServerError)
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
		templates.ExecuteTemplate(w, "generate.html", data)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// rsvpHandler fetches the RSVP by code and displays the RSVP page.
func rsvpHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var rsvp RSVP
	if err := rsvp.FindByCode(db, code); err != nil {
		log.Println("Could not find invite for code:", code, err)
		http.Error(w, "Invite not found", http.StatusNotFound)
		return
	}

	displayName := rsvp.Name
	if displayName == "" {
		displayName = "Friend"
	}

	data := struct {
		Name string
		Code string
	}{
		Name: displayName,
		Code: code,
	}

	templates.ExecuteTemplate(w, "rsvp.html", data)
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

		var rsvp RSVP
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

		http.Redirect(w, r, "/responses", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// responsesHandler lists all RSVPs.
func responsesHandler(w http.ResponseWriter, r *http.Request) {
	var allRSVPs []RSVP
	if err := db.Find(&allRSVPs).Error; err != nil {
		log.Println("Error fetching RSVPs:", err)
		http.Error(w, "Could not retrieve RSVPs", http.StatusInternalServerError)
		return
	}

	for i := range allRSVPs {
		if allRSVPs[i].Response == "" {
			allRSVPs[i].Response = "Pending"
		}
	}

	templates.ExecuteTemplate(w, "responses.html", allRSVPs)
}

func main() {
	initDatabase()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/generate", generateHandler)
	http.HandleFunc("/rsvp", rsvpHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/responses", responsesHandler)

	log.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
