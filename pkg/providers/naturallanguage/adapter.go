// Package naturallanguage implements the authenticated JSON parser-provider boundary.
package naturallanguage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/pkg/services"
)

const maximumResponseBytes = 64 * 1024

type Adapter struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func New(endpoint string, apiKey string, httpClient *http.Client) (*Adapter, error) {
	parsedEndpoint, parseError := url.Parse(endpoint)
	if parseError != nil || (parsedEndpoint.Scheme != "https" && parsedEndpoint.Scheme != "http") || parsedEndpoint.Host == "" || strings.TrimSpace(apiKey) == "" || httpClient == nil {
		return nil, errors.New("natural-language parser provider configuration is invalid")
	}
	return &Adapter{endpoint: endpoint, apiKey: apiKey, httpClient: httpClient}, nil
}

func (adapter *Adapter) Parse(ctx context.Context, request services.NaturalLanguageParseRequest) (services.NaturalLanguageParseResponse, error) {
	payload, marshalError := json.Marshal(request)
	if marshalError != nil {
		return services.NaturalLanguageParseResponse{}, errors.New("encode natural-language parser request")
	}
	httpRequest, requestError := http.NewRequestWithContext(ctx, http.MethodPost, adapter.endpoint, bytes.NewReader(payload))
	if requestError != nil {
		return services.NaturalLanguageParseResponse{}, errors.New("create natural-language parser request")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+adapter.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpResponse, responseError := adapter.httpClient.Do(httpRequest)
	if responseError != nil {
		return services.NaturalLanguageParseResponse{}, errors.New("natural-language parser request failed")
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return services.NaturalLanguageParseResponse{}, fmt.Errorf("natural-language parser returned status %d", httpResponse.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, maximumResponseBytes))
	decoder.DisallowUnknownFields()
	var parsed services.NaturalLanguageParseResponse
	if decodeError := decoder.Decode(&parsed); decodeError != nil {
		return services.NaturalLanguageParseResponse{}, services.ErrNaturalLanguageProviderResponseInvalid
	}
	var trailing any
	if trailingError := decoder.Decode(&trailing); !errors.Is(trailingError, io.EOF) {
		return services.NaturalLanguageParseResponse{}, services.ErrNaturalLanguageProviderResponseInvalid
	}
	return parsed, nil
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}
