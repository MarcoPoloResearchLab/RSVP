package horizon_test

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/pkg/config"
	horizonhandler "github.com/tyemirov/RSVP/pkg/handlers/horizon"
)

const (
	loopAwareSiteID    = "50bc3012-27c7-4fbd-8307-81dee1da50f7"
	loopAwareWidgetURL = "https://loopaware.mprlab.com/widget.js?site_id=" + loopAwareSiteID
	loopAwarePixelURL  = "https://loopaware.mprlab.com/pixel.js?site_id=" + loopAwareSiteID
)

func TestRenderedPagesUseCurrentLoopAwareSite(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	testsupport.LoadTemplates(testingContext)

	_, currentFilePath, _, sourceFound := runtime.Caller(0)
	if !sourceFound {
		testingContext.Fatal("locate LoopAware contract test")
	}
	landingTemplatePath := filepath.Join(filepath.Dir(currentFilePath), "..", "..", "..", config.TemplatesDir, config.TemplateLanding+config.TemplateExtension)
	landingTemplate, parseError := template.ParseFiles(landingTemplatePath)
	if parseError != nil {
		testingContext.Fatalf("parse landing page template: %v", parseError)
	}
	var landingHTML bytes.Buffer
	if executeError := landingTemplate.Execute(&landingHTML, map[string]interface{}{}); executeError != nil {
		testingContext.Fatalf("render landing page template: %v", executeError)
	}
	assertCurrentLoopAwareSite(testingContext, "landing page", landingHTML.String())

	owner := fixture.CreateUser(testsupport.OwnerUserID)
	horizonRequest := testsupport.Request(testingContext, http.MethodGet, config.WebHorizon, nil, &owner)
	horizonRequest.Header.Set("Accept", "text/html")
	horizonResponse := httptest.NewRecorder()
	horizonhandler.Handler(fixture.ApplicationContext, time.Now).ServeHTTP(horizonResponse, horizonRequest)
	if horizonResponse.Code != http.StatusOK {
		testingContext.Fatalf("application page status = %d, want %d", horizonResponse.Code, http.StatusOK)
	}
	assertCurrentLoopAwareSite(testingContext, "application page", horizonResponse.Body.String())
}

func assertCurrentLoopAwareSite(testingContext *testing.T, pageName string, responseBody string) {
	testingContext.Helper()
	for _, requiredURL := range []string{loopAwareWidgetURL, loopAwarePixelURL} {
		if strings.Count(responseBody, requiredURL) != 1 {
			testingContext.Fatalf("%s LoopAware URL %q count = %d, want 1", pageName, requiredURL, strings.Count(responseBody, requiredURL))
		}
	}
	if strings.Count(responseBody, "https://loopaware.mprlab.com/") != 2 {
		testingContext.Fatalf("%s LoopAware script count = %d, want 2", pageName, strings.Count(responseBody, "https://loopaware.mprlab.com/"))
	}
	if strings.Count(responseBody, loopAwareSiteID) != 2 {
		testingContext.Fatalf("%s LoopAware site identifier count = %d, want 2", pageName, strings.Count(responseBody, loopAwareSiteID))
	}
}
