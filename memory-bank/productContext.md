# QR RSVP Tracker

## 1. Overview
The **QR RSVP Tracker** is a web application that allows users to create and manage events, generate QR codes for invitees, track RSVP responses, and view confirmation status in a structured format. The system uses a unique code-based approach for each RSVP, making it easy to track responses without requiring invitees' email addresses.

## 2. Objectives
- Enable authenticated users to create and manage multiple events
- Generate unique QR codes and links for each invitee
- Allow invitees to RSVP (Yes/No) and indicate additional guests (+1, +2, etc.)
- Store and track RSVP responses in a database
- Provide an intuitive UI for creating events, managing invites, and viewing responses

## 3. Features

### 3.1 User Authentication
- Users authenticate via Google OAuth
- User profiles include email, name, and profile picture
- Authentication is required to create and manage events

### 3.2 Event Management
- Users can create multiple events with:
  - Title
  - Description
  - Start time
  - Duration (1-4 hours)
  - Optional invitees email
  - Optional invitees phone
- Events can be edited or deleted by their creator
- Each event has its own list of RSVPs

### 3.3 RSVP Generation
- For each event, users can create RSVPs by providing invitee names
- The system generates a unique code for each RSVP
- A QR code is generated containing the RSVP URL
- The RSVP can be distributed via link or QR code

### 3.4 RSVP Handling
- Each RSVP has a unique endpoint: `/rsvps/{code}`
- The RSVP page shows the invitee's name
- Invitees can select from multiple options:
  - Yes, with varying numbers of additional guests (+1, +2, +3, +4)
  - No
- The system updates the database with their response
- A thank you page is displayed after submission with an option to change the response

### 3.5 RSVP Tracking
- Event detail pages display a list of RSVPs and their status
- Fields include:
  - Name
  - Code
  - Response (Yes, No, or blank if pending)
  - Extra Guests count

## 4. Technical Requirements

### 4.1 Backend
- **Language:** Go
- **Framework:** Standard library `net/http` with custom routing
- **Database:** GORM with SQLite
- **Authentication:** Google OAuth via custom implementation
- **QR Generation:** QR code generation library

### 4.2 Data Models
- **User Model:**
  - Email (unique identifier)
  - Name
  - Profile picture URL
  - Relationship to Events (one-to-many)

- **Event Model:**
  - Title
  - Description
  - Start time
  - End time
  - Relationship to User (many-to-one)
  - Relationship to RSVPs (one-to-many)

- **RSVP Model:**
  - Name
  - Unique code
  - Response status
  - Extra guests count
  - Relationship to Event (many-to-one)

### 4.3 Frontend
- **Go's built-in template system** for rendering HTML
- **Bootstrap 4** for styling and responsive design
- HTML templates organized by feature:
  - Event templates (index, detail)
  - RSVP templates (form, responses)
  - Response templates (thank you)

### 4.4 QR Code and RSVP URLs
- RSVP URLs follow the pattern: `http://example.com/rsvps/{code}`
- QR codes are generated as base64-encoded images embedded directly in the page

## 5. User Roles
- **Authenticated User:**
  - Creates and manages events
  - Generates RSVPs for invitees
  - Views RSVP responses
  - Edits or deletes events

- **Invitee:**
  - Accesses RSVP link via QR code or direct URL
  - Responds with Yes/No and extra guests count
  - Can update their response

## 6. Deployment & Hosting
- Runs on a local server or cloud instance, port `8080`
- Supports both HTTP and HTTPS (with certificate and key files)
- Containerized with Docker for easy deployment

## 7. Future Enhancements
- Email integration for automated invites
- Enhanced analytics for event responses
- Multiple event types with customizable RSVP options
- Export RSVPs to CSV
- Additional personalization and styling options

## 8. Success Metrics
- Number of events created
- Number of RSVPs generated
- RSVP completion rate
- User retention and engagement

---

**Owner:** Vadym Tyemirov  
**Date:** 03/16/2025  
**Version:** 2.0
