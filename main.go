package main

import (
    "database/sql"
    "encoding/base64"
    "fmt"
    "html/template"
    "log"
    "net/http"

    "github.com/skip2/go-qrcode"
    _ "github.com/mattn/go-sqlite3"
)

// databaseSQL is the global database handle.
var databaseSQL *sql.DB

// templates is the global template cache. It parses all .html files in the templates folder.
var templates = template.Must(template.ParseGlob("templates/*.html"))

// initDatabase initializes the SQLite database and creates the RSVPs table if it doesn't exist.
func initDatabase() {
    var databaseError error
    databaseSQL, databaseError = sql.Open("sqlite3", "rsvps.db")
    if databaseError != nil {
        log.Fatal(databaseError)
    }

    createTableStatement := `
        CREATE TABLE IF NOT EXISTS rsvps (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT NOT NULL UNIQUE,
            response TEXT
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
        http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
    }
}

// rsvpHandler shows the RSVP form so an invitee can choose Yes or No.
func rsvpHandler(responseWriter http.ResponseWriter, request *http.Request) {
    inviteeEmail := request.URL.Query().Get("email")
    // Pass the email to rsvp.html so we know whom we're updating.
    templates.ExecuteTemplate(responseWriter, "rsvp.html", inviteeEmail)
}

// submitHandler updates the invitee's response in the database.
func submitHandler(responseWriter http.ResponseWriter, request *http.Request) {
    if request.Method == http.MethodPost {
        inviteeEmail := request.FormValue("email")
        inviteeResponse := request.FormValue("response")

        _, databaseError := databaseSQL.Exec(
            "UPDATE rsvps SET response = ? WHERE email = ?",
            inviteeResponse, inviteeEmail,
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
    rows, queryError := databaseSQL.Query("SELECT name, email, response FROM rsvps")
    if queryError != nil {
        log.Fatal(queryError)
    }
    defer rows.Close()

    // A struct to represent the RSVP row
    type rsvpRow struct {
        Name     string
        Email    string
        Response string
    }

    var allRSVPs []rsvpRow
    for rows.Next() {
        var nameValue, emailValue, responseValue string
        scanError := rows.Scan(&nameValue, &emailValue, &responseValue)
        if scanError != nil {
            log.Println(scanError)
            continue
        }
        if responseValue == "" {
            responseValue = "Pending"
        }
        allRSVPs = append(allRSVPs, rsvpRow{
            Name:     nameValue,
            Email:    emailValue,
            Response: responseValue,
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
