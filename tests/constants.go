package tests

// TestUserData contains the standard test user information used across all tests.
type TestUserData struct {
	Email   string
	Name    string
	Picture string
}

// DefaultTestUser is the standard test user used across all tests.
var DefaultTestUser = TestUserData{
	Email:   "test@example.com",
	Name:    "Test User",
	Picture: "https://example.com/picture.jpg",
}
