# RSVP

RSVP shows events, calendar lanes, attention probes, and invitations in the Horizon view.
The Go backend uses SQLite.
TAuth owns browser authentication.
The backend uses the official LLM Proxy client for natural-language drafts.

## Local Application

1. Supply `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in the private `.env.docker` file.
2. Add `http://localhost:8080` to the Google client origins.
3. Run `make up`.
4. Open `http://localhost:8080/`.
5. Use the shared Google sign-in control.
6. Run `make down` to stop the services.

The local stack preserves its database volumes and private keys between starts.
The local LLM Proxy fixture creates deterministic open-lane proposals.
It does not use a live language model.

## Validation

Run `make ci` for Go validation.
Run `make browser-test` for desktop and mobile browser validation.

Use the [operator runbook](OPERATOR_RUNBOOK.md) for configuration and production operations.
Use the [user guide](USER_GUIDE.md) for organizer tasks.
Use the [architecture](ARCHITECTURE.md) for resource and provider contracts.
