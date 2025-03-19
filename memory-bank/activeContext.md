# Active Context

## Current Focus

The current focus is on code refactoring and enhancing the QR RSVP Tracker application. The core functionality is implemented, but there are significant opportunities for improvement in the following areas:

1. **Code Quality**: Implementing strict code quality standards to improve maintainability and reliability
   - **Integration Testing**: Implementing proper integration tests without mocks
   - **Variable Naming**: Replacing short variable names with descriptive ones
   - **Error Handling**: Ensuring all errors are properly handled and never swallowed
   - **String Constants**: Moving string literals to constant definitions
   - **Code Duplication**: Eliminating redundant code through abstraction and reuse

2. **User Experience**: Enhancing the UI/UX for both event organizers and invitees
   - **Single-Page Functionality**: Ensuring each page handles all related operations without navigation
   - **In-Page Forms**: Implementing forms that appear within the same page rather than navigating to separate pages
   - **Minimal JavaScript**: Using HTML links and forms with minimal JavaScript when possible
   - **Consistent Entity Management Pattern**: Both Events and RSVPs pages follow identical UI/UX patterns:
     - **List View Always Present**: The entity list (events or RSVPs) always remains visible on screen
     - **Empty State Handling**: When no entities exist, an "Add" button (for events) or "New" button (for RSVPs) is prominently displayed
     - **In-Page Form Display**: When the Add/New button is clicked, a form appears above the entity table
     - **Comprehensive Form Controls**: All forms display complete entity details and provide consistent Cancel, Delete, and Update buttons
     - **Persistent Context**: Users never lose sight of their entity list, maintaining context while performing actions

3. **Code Organization**: Refining the structure and patterns for better maintainability

4. **Feature Completeness**: Adding missing features from the original requirements

## Recent Changes

### URL Path Consistency Fix for Form Submissions
- Fixed a critical bug with event deletion and form submissions
- Identified and fixed inconsistent URL patterns across the application
- Standardized all form submissions to use trailing slashes (`/events/` instead of `/events`)
- Modified forms to use form data instead of query parameters for better reliability
- Implemented comprehensive tests to verify different form submission patterns
- Updated documentation to clearly define URL and form submission patterns
- Added detailed explanation in systemPatterns.md about the trailing slash requirement
- Documented how form redirects can cause data loss when form parameters aren't properly retained

The critical issue was that when browsers submit forms to URLs without trailing slashes, the server redirects with a 301 to the trailing slash version. During this redirect, the browser changes the request from POST to GET, causing all form data (including the DELETE method override) to be lost. This silently prevented event deletion from working.

### Comprehensive Integration Testing Implementation
- Implemented a comprehensive integration test suite covering all aspects of the system
- Created specialized test files for different testing concerns:
  - `relationship_test.go`: Tests for event-RSVP relationships and cascade operations
  - `validation_test.go`: Tests for input validation for events and RSVPs
  - `authorization_test.go`: Tests for access control and ownership verification
  - `delegation_test.go`: Tests for event ownership transfer between users
  - `edge_cases_test.go`: Tests for special cases and error handling
- Added tests for key scenarios:
  - Event delegation with RSVP preservation
  - Cascade deletion of RSVPs when events are deleted
  - RSVP response changes with various guest counts
  - Authorization checks for events and RSVPs
  - Handling of special characters and edge cases
- Ensured all tests follow the project's code quality standards
- Updated documentation to reflect the comprehensive testing approach

### Event Model Duration Calculation Fix
- Added a proper `DurationHours` method to the Event model to calculate the duration in hours between start and end times
- The method is essential for the UI templates which expect to access this information when displaying events
- Fixed inconsistencies between how backend handlers (which calculate end time based on start time + duration from forms) and UI templates (which need to extract duration from stored start/end times) interact
- Ensured `ShowHandler` correctly passes all required data fields to templates, including properly structured Events array
- Fixed UI test cases to correctly match the actual application flow (e.g., RSVP forms exist on RSVP pages, not event detail pages)
- Updated test variable naming conventions to be consistent and descriptive (avoiding single letter variables like `t`)

### Integration Test Improvements
- Enhanced the `Cleanup` method in integration tests to properly remove temporary database files
- Improved error handling in test cleanup to ensure resources are properly released
- Added detailed logging for test database cleanup operations
- Fixed an issue where temporary test databases were not being properly deleted after tests
- Ensured compliance with project code quality standards by using descriptive variable names
- Moved integration tests to project root for better organization
- Restructured tests to be individually runnable without wrapper functions
- Eliminated redundant test directory structure (tests/integration -> tests/)
- Created a shared test utilities package to eliminate code duplication
- Centralized test user data in a single location for consistency
- Removed redundant TestEventOperations and TestRSVPOperations wrapper functions
- Eliminated unnecessary main_test.go file
- Modified database path handling to store test databases in the system's temporary directory
- Enhanced test cleanup to search in multiple directories for database files
- Added more robust error handling and logging for test database cleanup operations

### Test Files Overview
The project includes a comprehensive suite of tests that verify different aspects of functionality:

| Test File | Purpose |
| --- | --- |
| `authorization_test.go` | Tests for access control mechanisms, ownership verification, and security boundaries |
| `delegation_test.go` | Tests for event ownership transfer functionality between users |
| `edge_cases_test.go` | Tests for handling special cases, boundary conditions, and error scenarios |
| `event_edit_form_test.go` | Tests for the event editing UI components and form functionality |
| `event_test.go` | Tests for core event CRUD operations and related functionality |
| `form_integration_test.go` | Tests for form submissions and the integration between UI forms and backend handlers |
| `relationship_test.go` | Tests for event-RSVP relationships, cascading operations, and data integrity |
| `rsvp_test.go` | Tests for core RSVP CRUD operations and related functionality |
| `ui_interaction_test.go` | Tests for user interface interactions, navigation flows, and UI component behavior |
| `validation_test.go` | Tests for input validation rules, error messages, and validation error handling |

### RESTful API Reorganization
- Restructured handlers to follow RESTful naming conventions (list, create, show, update, delete)
- Implemented flat routing structure with query parameters instead of path parameters
- Organized handlers by resource type (event, rsvp)
- Simplified routing logic with dedicated router files for each resource
- Removed redundant code and consolidated functionality

### Authentication System
- Implemented Google OAuth authentication
- Added user profile information display
- Created session management

### Multi-Event Support
- Restructured the application to support multiple events per user
- Added event management UI (create, edit, delete)
- Implemented event detail views

### RSVP System
- Updated RSVP model to use unique codes
- Enhanced RSVP response options (+1, +2, +3, +4 guests)
- Added thank you page with option to change response
- Improved navigation between events and RSVPs with direct links
- Redesigned RSVP list page to match the events page layout
- Added RSVP editing functionality similar to event editing

### UX Enhancement Implementation
- Created four HTML files for improved user experience:
  - `events.html`: Single-page interface for managing events (create, list, edit, update, delete)
  - `rsvps.html`: Single-page interface for managing RSVPs (create, list, edit, update, delete)
  - `rsvp/visualization.html`: Print-ready visualization with QR code for sharing
  - `rsvp/response.html`: Unprotected form for invitees to respond to RSVPs
- Implemented RESTful routing with query parameters:
  - Protected routes for event and RSVP management (`/events`, `/rsvps`)
  - Unprotected route for RSVP responses (`/rsvp?code={code}`)
- Added QR code visualization capabilities with properly encoded URLs
- Implemented Bootstrap-based responsive design across all pages
- Maintained existing patterns for UI consistency and code quality

### Authentication Handling Improvements
- Removed redundant authentication checks throughout the codebase
- Updated `RequireAuthentication` method in base handler to remove redirect logic
- Ensured proper reliance on the Google authentication middleware
- Standardized authentication handling across all handlers
- Added proper ownership verification for events (users can only view/edit/delete their own events)
- Improved code consistency with descriptive variable names and proper error handling
- Eliminated duplicate authentication logic while maintaining security

## Active Decisions

### Database Schema
The current schema uses GORM with SQLite, which is suitable for the current scale. As the application grows, we may need to consider:
- Migration to a more robust database (PostgreSQL, MySQL)
- Optimization of queries for performance
- Enhanced indexing strategy

### Database Reset Behavior
The current implementation drops and recreates all tables on every application start:
- This approach is suitable for early development but not for production
- We need to modify the `InitDatabase` function in `pkg/services/database.go` to:
  - Preserve existing data between application restarts
  - Use migrations for schema changes instead of dropping tables
  - Allow manual database resets when needed, rather than automatic ones

### ID Generation System
The application uses a sophisticated ID generation system:
- Base62 encoding (0-9, A-Z, a-z) for User and Event models
- Base36 encoding (0-9, A-Z) for RSVP models to make them more user-friendly
- All IDs are 8 characters long and cryptographically secure
- Uniqueness is verified within each table before assignment
- We need to implement cross-event uniqueness checks for RSVP codes

### UI Framework
Bootstrap 4 is currently used for styling. We're considering:
- Upgrading to Bootstrap 5
- Implementing a more consistent design system
- Adding more interactive elements with JavaScript

### Deployment Strategy
The application can be deployed as a standalone binary or containerized with Docker. We're evaluating:
- Cloud deployment options
- CI/CD pipeline setup
- Monitoring and logging solutions

## Next Steps

### Short-term (Next 2 Weeks)
1. Add email notification capability for new RSVPs
2. Implement RSVP statistics and dashboard
3. Add export functionality for RSVP data (CSV, PDF)

### Medium-term (Next 1-2 Months)
1. Enhance mobile responsiveness
2. Add customization options for event pages
3. Implement event templates for quick creation

### Long-term (3+ Months)
1. Explore integration with calendar systems
2. Add support for recurring events
3. Implement advanced analytics for event organizers

## Current Challenges

1. **QR Code Distribution**: Need better ways to distribute QR codes (email, social media)
2. **RSVP Tracking**: Improving the visualization of RSVP status
3. **User Onboarding**: Simplifying the process for new users

---

**Last Updated:** 03/17/2025 (Updated with Event model duration calculation fixes and test improvements)
