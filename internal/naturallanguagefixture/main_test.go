package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerReturnsDeterministicOpenLane(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, fixtureParsePath, strings.NewReader(`{"input_text":"Wait for the permit","reference_time":"2030-01-02T15:00:00Z","timezone":"America/Los_Angeles"}`))
	request.Header.Set("Authorization", "Bearer "+fixtureAPIKey)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	want := `"mode":"open_lane"`
	if !strings.Contains(response.Body.String(), want) {
		t.Fatalf("body = %q, want %q", response.Body.String(), want)
	}
}

func TestHandlerRejectsMissingCredential(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, fixtureParsePath, strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
