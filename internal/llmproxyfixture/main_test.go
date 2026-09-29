package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerReturnsDeterministicOpenLane(t *testing.T) {
	t.Setenv("LLM_PROXY_SECRET", "fixture-secret")
	request := httptest.NewRequest(http.MethodPost, fixtureParsePath+"?key=fixture-secret", strings.NewReader(`{"messages":[{"role":"system","content":"instructions"},{"role":"user","content":"{\"input_text\":\"Wait for the permit\",\"reference_time\":\"2030-01-02T15:00:00Z\",\"timezone\":\"America/Los_Angeles\"}"}],"reasoning_effort":"low"}`))
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
	t.Setenv("LLM_PROXY_SECRET", "fixture-secret")
	request := httptest.NewRequest(http.MethodPost, fixtureParsePath, strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
