# QR RSVP Tracker

## 1. Overview

The **QR RSVP Tracker** is a simple web application that allows event organizers to generate QR codes for invitees, track their RSVP responses, and view the confirmation status in a structured format.

## 2. Objectives

- Enable event organizers to create unique QR codes for each invitee.
- Allow invitees to RSVP via a simple Yes/No interface.
- Store and track RSVP responses in a database.
- Provide an intuitive UI for managing invitations and responses.

## 3. Features

### 3.1 Invite Generation

- Form for entering invitee name and email.
- Generates a unique QR code linked to an RSVP URL.
- Stores invitee details in a database.
- Displays QR code and RSVP URL for sharing.

### 3.2 RSVP Handling

- Invitees scan their QR code or click the link.
- Page displays RSVP form with Yes/No buttons.
- Submission updates the database with their response.
- Redirects invitee to a confirmation message.

### 3.3 RSVP Tracking

- Admin page displays a list of invitees and their RSVP status.
- Status options: **Pending**, **Yes**, **No**.
- Responses update in real time.

## 4. Technical Requirements

### 4.1 Backend

- Language: Go
- Framework: `net/http`
- Database: SQLite

### 4.2 Frontend

- Uses Go’s built-in templating system.
- HTML templates stored in `templates/` directory.
- Pages:
  - `index.html` – Invitation form
  - `generate.html` – QR code display
  - `rsvp.html` – RSVP submission page
  - `responses.html` – RSVP status tracking

### 4.3 QR Code Generation

- Uses `github.com/skip2/go-qrcode`.
- Encodes unique RSVP URLs.
- Stores QR codes as base64 PNG images.

## 5. User Roles

- **Event Organizer:** Can generate QR codes, send invitations, and track responses.
- **Invitee:** Can RSVP via QR scan or URL.

## 6. Deployment & Hosting

- Runs on a local server or cloud instance.
- Exposed on port `8080`.
- Can be containerized using Docker.

## 7. Future Enhancements

- Email integration for automated invites.
- Admin login for restricted access.
- Event customization (multiple events support).
- Export RSVP list as CSV.

## 8. Success Metrics

- Number of QR codes generated.
- RSVP completion rate.
- Response accuracy and reliability.

---

**Owner:** Vadym Tyemirov
**Date:** 02/28/2025
**Version:** 1.0

