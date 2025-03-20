# RSVP 

RSVP is an events invitation platform that relies on physical QR Codes and allows printing, sending and tracking invitations to events.

## Running Tests

The application uses integration tests to verify functionality. Tests are located in the `tests/` directory at the project root.

### Running All Tests

To run all tests:

```shell
go test ./tests/... -v
```

### Running Specific Test Packages

To run only integration tests:

```shell
go test ./tests/integration -v
```

### Running Individual Tests

Each test function is individually runnable:

```shell
go test ./tests/integration -run TestCreateEvent -v
```

You can also run all tests in a file:

```shell
go test ./tests/integration/event_test.go -v
```

### Test Structure

- `tests/` - Contains all integration tests
  - `event_test.go` - Basic CRUD tests for events
  - `rsvp_test.go` - Basic CRUD tests for RSVPs
  - `relationship_test.go` - Tests for event-RSVP relationships and cascade operations
  - `validation_test.go` - Tests for input validation
  - `authorization_test.go` - Tests for access control
  - `delegation_test.go` - Tests for event ownership transfer
  - `edge_cases_test.go` - Tests for special cases and error handling
- `tests/routes/` - Contains test route configurations
- `tests/testutils/` - Contains shared test utilities and constants

### Test Design Principles

The tests follow these key design principles:

1. **Real Dependencies**: Integration tests use real dependencies without mocks, ensuring the entire system works together correctly.

2. **Shared Test Data**: Common test data like the test user is defined once in `testutils/constants.go` and reused across all tests to avoid duplication.

3. **Proper Test Isolation**: Each test creates its own database and test servers, ensuring tests don't interfere with each other.

4. **Complete Test Lifecycle**: Tests properly clean up resources after completion, including removing temporary database files.

5. **Comprehensive Coverage**: Tests cover all aspects of the system:
   - Basic CRUD operations for events and RSVPs
   - Relationships between events and RSVPs
   - Cascade operations (e.g., deleting RSVPs when events are deleted)
   - Input validation for both events and RSVPs
   - Authorization and access control
   - Event delegation and ownership transfer
   - Edge cases and error handling

6. **Verification of Updates**: The update tests verify that existing records are properly updated rather than creating new ones.

7. **Relationship Testing**: Tests verify that RSVPs are correctly attached to existing events and that relationships are maintained during operations like delegation.

8. **Descriptive Test Names**: Each test is named to clearly indicate what functionality it's testing, making it easier to identify failures.

## SSL Certificate Setup
This app supports HTTPS (TLS) with certificates for both local development and production.

### Local Development (localhost)
For local testing with trusted certificates, use mkcert.

Install mkcert:
```shell
brew install mkcert
mkcert -install
```

Generate certificates:

```shell
mkcert localhost 127.0.0.1 ::1
```

```shell
certs/localhost.pem
certs/localhost-key.pem
```

Set environment variables:

```shell
export TLS_CERT_PATH=certs/localhost.pem
export TLS_KEY_PATH=certs/localhost-key.pem
```

Production (public domain)
For production deployments using a real domain (rsvp.mprlab.com), use Let's Encrypt.

Steps:
On your Mac, install Certbot:

```shell
brew install certbot
```

Obtain a certificate via DNS challenge:

```shell
sudo certbot certonly --manual --preferred-challenges dns -d rsvp.mprlab.com
```

After success, certificates are stored in:

```shell
/etc/letsencrypt/live/mywebsite.com/fullchain.pem
/etc/letsencrypt/live/mywebsite.com/privkey.pem
```

Copy the certificates to the production server:

```shell
scp /etc/letsencrypt/live/mywebsite.com/fullchain.pem user@server:/opt/myapp/certs/fullchain.pem
scp /etc/letsencrypt/live/mywebsite.com/privkey.pem user@server:/opt/myapp/certs/privkey.pem
```

On the production server, set environment variables:

```shell
export TLS_CERT_PATH=/opt/myapp/certs/fullchain.pem
export TLS_KEY_PATH=/opt/myapp/certs/privkey.pem
```
