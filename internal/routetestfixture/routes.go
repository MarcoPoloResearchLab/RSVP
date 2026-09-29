// Package routetestfixture constructs real application routers for HTTP tests.
package routetestfixture

import (
	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
	"testing"
)

// New constructs a router with the isolated environment profile.
func New(t *testing.T, application *config.ApplicationContext) *routes.Routes {
	t.Helper()
	result, err := routes.New(application, testsupport.EnvironmentConfig())
	if err != nil {
		t.Fatal(err)
	}
	return result
}
