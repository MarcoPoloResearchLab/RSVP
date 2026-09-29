package server_test

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/pkg/server"
)

func TestHTTPServerClosesIncompleteHeaders(testingHandle *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "HTTP"
		if secure {
			name = "HTTPS"
		}
		testingHandle.Run(name, func(testingHandle *testing.T) {
			testingHandle.Parallel()
			handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			})
			listener := httptest.NewUnstartedServer(handler)
			listener.Config = server.NewHTTPServer(listener.Listener.Addr().String(), handler)
			if secure {
				listener.StartTLS()
			} else {
				listener.Start()
			}
			testingHandle.Cleanup(listener.Close)
			dialer := &net.Dialer{Timeout: time.Second}
			var connection net.Conn
			var dialError error
			if secure {
				roots := x509.NewCertPool()
				roots.AddCert(listener.Certificate())
				connection, dialError = tls.DialWithDialer(dialer, "tcp", listener.Listener.Addr().String(), &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})
			} else {
				connection, dialError = dialer.Dial("tcp", listener.Listener.Addr().String())
			}
			if dialError != nil {
				testingHandle.Fatalf("connect: %v", dialError)
			}
			testingHandle.Cleanup(func() {
				if closeError := connection.Close(); closeError != nil {
					testingHandle.Errorf("close connection: %v", closeError)
				}
			})
			if deadlineError := connection.SetDeadline(time.Now().Add(6 * time.Second)); deadlineError != nil {
				testingHandle.Fatalf("set deadline: %v", deadlineError)
			}
			if _, writeError := io.WriteString(connection, "GET / HTTP/1.1\r\nHost: localhost\r\nX-Partial: "); writeError != nil {
				testingHandle.Fatalf("write incomplete request: %v", writeError)
			}
			var received [1]byte
			_, readError := connection.Read(received[:])
			var networkError net.Error
			if errors.As(readError, &networkError) && networkError.Timeout() {
				testingHandle.Fatal("server kept incomplete headers open past the header deadline")
			}
			if !errors.Is(readError, io.EOF) {
				testingHandle.Fatalf("expected closed connection, got %v", readError)
			}
			request, requestError := http.NewRequest(http.MethodPost, listener.URL, strings.NewReader("ordinary body"))
			if requestError != nil {
				testingHandle.Fatalf("create ordinary request: %v", requestError)
			}
			response, responseError := listener.Client().Do(request)
			if responseError != nil {
				testingHandle.Fatalf("ordinary request: %v", responseError)
			}
			if closeError := response.Body.Close(); closeError != nil {
				testingHandle.Fatalf("close response: %v", closeError)
			}
			if response.StatusCode != http.StatusNoContent {
				testingHandle.Fatalf("ordinary response status: %d", response.StatusCode)
			}
		})
	}
}

func TestHTTPServerAllowsBodyAfterHeaderDeadline(testingHandle *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, readError := io.ReadAll(request.Body)
		if readError != nil || string(body) != "{}" {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	listener := httptest.NewUnstartedServer(handler)
	listener.Config = server.NewHTTPServer(listener.Listener.Addr().String(), handler)
	listener.Start()
	testingHandle.Cleanup(listener.Close)
	connection, dialError := net.DialTimeout("tcp", listener.Listener.Addr().String(), time.Second)
	if dialError != nil {
		testingHandle.Fatalf("connect: %v", dialError)
	}
	testingHandle.Cleanup(func() {
		if closeError := connection.Close(); closeError != nil {
			testingHandle.Errorf("close connection: %v", closeError)
		}
	})
	if deadlineError := connection.SetDeadline(time.Now().Add(8 * time.Second)); deadlineError != nil {
		testingHandle.Fatalf("set deadline: %v", deadlineError)
	}
	if _, writeError := io.WriteString(connection, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 2\r\n\r\n"); writeError != nil {
		testingHandle.Fatalf("write headers: %v", writeError)
	}
	time.Sleep(5200 * time.Millisecond)
	if _, writeError := io.WriteString(connection, "{}"); writeError != nil {
		testingHandle.Fatalf("write body: %v", writeError)
	}
	response, responseError := http.ReadResponse(bufio.NewReader(connection), nil)
	if responseError != nil {
		testingHandle.Fatalf("read response: %v", responseError)
	}
	if closeError := response.Body.Close(); closeError != nil {
		testingHandle.Fatalf("close response: %v", closeError)
	}
	if response.StatusCode != http.StatusNoContent {
		testingHandle.Fatalf("ordinary slow body rejected: %d", response.StatusCode)
	}
}
