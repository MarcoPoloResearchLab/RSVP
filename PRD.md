# QR RSVP Tracker

## 1. Overview
The **QR RSVP Tracker** is a simple web application that allows event organizers to generate QR codes for invitees, track their RSVP responses, and view confirmation status in a structured format. This revised version supports cases in which the organizer does not have invitees’ email addresses, using a **small Base36 ID** instead.

## 2. Objectives
- Enable event organizers to create unique invites for guests.
- Generate a short Base36 identifier for each invite if email is not known.
- Allow invitees to RSVP (Yes/No) and indicate additional guests (+1, +2, etc.).
- Store and track RSVP responses in a database.
- Provide an intuitive UI for creating invites and viewing responses.

## 3. Features

### 3.1 Invite Generation
- **Option A (Email known):** If an email is provided, the system will insert or update the invitee record using that email.
- **Option B (No email):** If no email is provided, the system generates a **Base36 ID** and stores that as the unique identifier.
- The system creates a unique QR code URL using either the email or the Base36 ID.
- The invite can be distributed via link or QR code.

### 3.2 RSVP Handling
- Each invite has an RSVP endpoint: `/rsvp?identifier=...`
  - If the identifier is an email, the system uses email to look up the record.
  - If the identifier is a Base36 ID, the system looks it up accordingly.
- The RSVP page shows the invitee’s name (if known) or a placeholder.
- Invitees can click buttons for **Yes** or **No**, and indicate how many additional guests they are bringing.
- The system updates the database with their response.

### 3.3 RSVP Tracking
- An admin page displays a list of invitees and their RSVP status.
- Fields include:
  - **Name** (if provided)
  - **Email** or **Base36 ID**
  - **Response** (Yes, No, Pending)
  - **Extra Guests** (+0, +1, …)
- Sorting or search capabilities could be added later.

## 4. Technical Requirements

### 4.1 Backend
- **Language:** Go
- **Framework:** `net/http`
- **Database:** SQLite
- **QR Generation:** `github.com/skip2/go-qrcode`

### 4.2 Unique Identifiers
- The system can generate a short **Base36** string (e.g., `k28f`) for each invite when no email is supplied.
  - This could be done with a function that converts an integer sequence to Base36.
- The database table should store either an email **OR** a Base36 ID, with at least one guaranteed to be unique.
- The application must handle either approach.

### 4.3 Frontend
- **Go’s built-in template system** for rendering HTML.
- HTML templates in `templates/` folder:
  - `index.html` – Invitation form, letting the organizer provide a name, optional email, and generate an invite.
  - `generate.html` – Shows the resulting QR code and RSVP link.
  - `rsvp.html` – Form for invitees to respond with Yes/No and additional guests.
  - `responses.html` – List of all invitees and their RSVP statuses.

### 4.4 QR Code and RSVP URLs
- If email is provided: `http://example.com/rsvp?identifier=email@example.com`
- If no email is provided, generate a Base36 ID (e.g., `abc123`): `http://example.com/rsvp?identifier=abc123`

## 5. User Roles
- **Event Organizer:**
  - Creates invites by optionally providing name/email.
  - Shares QR codes or links.
  - Views RSVPs.
- **Invitee:**
  - Visits RSVP link.
  - Responds with Yes/No and extra guests.

## 6. Deployment & Hosting
- Runs on a local server or cloud instance, port `8080`.
- Could be containerized.

## 7. Future Enhancements
- Email integration for automated invites if email is known.
- Admin login for restricted access.
- Multiple concurrent events.
- Export RSVPs to CSV.
- Additional personalization and style.

## 8. Success Metrics
- Number of invites generated.
- RSVP completion rate.
- Timely updates to the RSVP list.


---

**Owner:** Vadym Tyemirov
**Date:** 02/28/2025
**Version:** 1.0

