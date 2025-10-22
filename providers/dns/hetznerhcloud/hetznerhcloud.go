package hetznerhcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/log"
	"github.com/go-acme/lego/v4/platform/config/env"
)

const (
	envNamespace = "HCLOUD_"

	EnvToken   = envNamespace + "TOKEN"
	EnvBaseURL = envNamespace + "BASE_URL"

	EnvTTL                = envNamespace + "TTL"
	EnvPropagationTimeout = envNamespace + "PROPAGATION_TIMEOUT"
	EnvPollingInterval    = envNamespace + "POLLING_INTERVAL"
	EnvHTTPTimeout        = envNamespace + "HTTP_TIMEOUT"

	defaultBaseURL = "https://api.hetzner.cloud"
	defaultTTL     = 60

	maxRetries = 3
)

var _ challenge.ProviderTimeout = (*DNSProvider)(nil)

// Config is used to configure the creation of the DNSProvider.
type Config struct {
	Token              string
	BaseURL            string
	TTL                int
	PropagationTimeout time.Duration
	PollingInterval    time.Duration
	HTTPClient         *http.Client
}

// NewDefaultConfig returns a default configuration.
func NewDefaultConfig() *Config {
	return &Config{
		BaseURL:            env.GetOrDefaultString(EnvBaseURL, defaultBaseURL),
		TTL:                env.GetOrDefaultInt(EnvTTL, defaultTTL),
		PropagationTimeout: env.GetOrDefaultSecond(EnvPropagationTimeout, dns01.DefaultPropagationTimeout),
		PollingInterval:    env.GetOrDefaultSecond(EnvPollingInterval, dns01.DefaultPollingInterval),
		HTTPClient: &http.Client{
			Timeout: env.GetOrDefaultSecond(EnvHTTPTimeout, 30*time.Second),
		},
	}
}

// NewDNSProvider returns a DNSProvider instance configured from the environment.
func NewDNSProvider() (*DNSProvider, error) {
	values, err := env.Get(EnvToken)
	if err != nil {
		return nil, fmt.Errorf("hetznerhcloud: %w", err)
	}

	config := NewDefaultConfig()
	config.Token = values[EnvToken]

	return NewDNSProviderConfig(config)
}

// NewDNSProviderConfig returns a DNSProvider instance configured for Hetzner Cloud DNS.
func NewDNSProviderConfig(config *Config) (*DNSProvider, error) {
	if config == nil {
		return nil, errors.New("hetznerhcloud: the configuration of the DNS provider is nil")
	}

	if config.Token == "" {
		return nil, errors.New("hetznerhcloud: HCLOUD_TOKEN is missing")
	}

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: env.GetOrDefaultSecond(EnvHTTPTimeout, 30*time.Second)}
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("hetznerhcloud: %w", err)
	}

	provider := &DNSProvider{
		config:         config,
		baseURL:        baseURL,
		recordIDs:      make(map[string]string),
		zoneIDs:        make(map[string]string),
		findZoneByFqdn: dns01.FindZoneByFqdn,
	}

	return provider, nil
}

// DNSProvider implements the challenge.Provider interface.
type DNSProvider struct {
	config  *Config
	baseURL *url.URL

	recordMu  sync.Mutex
	recordIDs map[string]string

	zoneMu  sync.Mutex
	zoneIDs map[string]string

	findZoneByFqdn func(string) (string, error)
}

// Timeout returns the timeout and interval to use when checking for DNS propagation.
func (d *DNSProvider) Timeout() (timeout, interval time.Duration) {
	return d.config.PropagationTimeout, d.config.PollingInterval
}

// Present creates a TXT record using the specified parameters.
func (d *DNSProvider) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	authZone, err := d.findZoneByFqdn(info.EffectiveFQDN)
	if err != nil {
		return fmt.Errorf("hetznerhcloud: could not find zone for domain %q: %w", domain, err)
	}

	zoneName := dns01.UnFqdn(authZone)

	ctx := context.Background()

	zoneID, err := d.getZoneID(ctx, zoneName)
	if err != nil {
		return err
	}

	fqdn := dns01.UnFqdn(info.EffectiveFQDN)
	relativeRecord := fqdn
	suffix := "." + zoneName
	fqdnLower := strings.ToLower(fqdn)
	suffixLower := strings.ToLower(suffix)

	switch {
	case strings.EqualFold(fqdn, zoneName):
		relativeRecord = ""
	case len(fqdn) > len(suffix) && strings.HasSuffix(fqdnLower, suffixLower):
		relativeRecord = fqdn[:len(fqdn)-len(suffix)]
	}

	if relativeRecord == "" {
		relativeRecord = "_acme-challenge"
	}

	payload := map[string]any{
		"name":  relativeRecord,
		"type":  "TXT",
		"value": info.Value,
		"ttl":   d.config.TTL,
	}

	var response struct {
		Record struct {
			ID json.RawMessage `json:"id"`
		} `json:"record"`
	}

	if err = d.post(ctx, fmt.Sprintf("/v1/zones/%s/records", zoneID), payload, &response); err != nil {
		return err
	}

	recordID, err := parseIdentifier(response.Record.ID)
	if err != nil {
		return fmt.Errorf("hetznerhcloud: %w", err)
	}

	d.recordMu.Lock()
	d.recordIDs[strings.ToLower(info.EffectiveFQDN)] = recordID
	d.recordMu.Unlock()

	return nil
}

// CleanUp removes the TXT record matching the specified parameters.
func (d *DNSProvider) CleanUp(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	authZone, err := d.findZoneByFqdn(info.EffectiveFQDN)
	if err != nil {
		return fmt.Errorf("hetznerhcloud: could not find zone for domain %q: %w", domain, err)
	}

	zoneName := dns01.UnFqdn(authZone)

	d.recordMu.Lock()
	recordID, ok := d.recordIDs[strings.ToLower(info.EffectiveFQDN)]
	d.recordMu.Unlock()

	if !ok {
		return nil
	}

	ctx := context.Background()

	zoneID, err := d.getZoneID(ctx, zoneName)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/zones/%s/records/%s", zoneID, recordID)
	if err := d.delete(ctx, path); err != nil {
		return err
	}

	d.recordMu.Lock()
	delete(d.recordIDs, strings.ToLower(info.EffectiveFQDN))
	d.recordMu.Unlock()

	return nil
}

func (d *DNSProvider) getZoneID(ctx context.Context, zoneName string) (string, error) {
	zoneKey := strings.ToLower(zoneName)

	d.zoneMu.Lock()
	if id, ok := d.zoneIDs[zoneKey]; ok {
		d.zoneMu.Unlock()
		return id, nil
	}
	d.zoneMu.Unlock()

	page := 1
	for {
		query := url.Values{}
		query.Set("name", zoneName)
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", "50")

		var response struct {
			Zones []struct {
				ID   json.RawMessage `json:"id"`
				Name string          `json:"name"`
			} `json:"zones"`
			Meta struct {
				Pagination struct {
					NextPage *int `json:"next_page"`
				} `json:"pagination"`
			} `json:"meta"`
		}

		if err := d.get(ctx, "/v1/zones", query, &response); err != nil {
			return "", err
		}

		for _, zone := range response.Zones {
			if strings.EqualFold(zone.Name, zoneName) {
				id, err := parseIdentifier(zone.ID)
				if err != nil {
					return "", fmt.Errorf("hetznerhcloud: %w", err)
				}

				d.zoneMu.Lock()
				d.zoneIDs[zoneKey] = id
				d.zoneMu.Unlock()
				return id, nil
			}
		}

		nextPage := 0
		if response.Meta.Pagination.NextPage != nil {
			nextPage = *response.Meta.Pagination.NextPage
		}

		if nextPage == 0 {
			break
		}

		page = nextPage
	}

	return "", fmt.Errorf("hetznerhcloud: zone %q not found", zoneName)
}

func (d *DNSProvider) get(ctx context.Context, path string, query url.Values, into any) error {
	return d.do(ctx, http.MethodGet, path, query, nil, into)
}

func (d *DNSProvider) post(ctx context.Context, path string, payload any, into any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("hetznerhcloud: %w", err)
	}

	return d.do(ctx, http.MethodPost, path, nil, body, into)
}

func (d *DNSProvider) delete(ctx context.Context, path string) error {
	return d.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (d *DNSProvider) do(ctx context.Context, method, path string, query url.Values, body []byte, into any) error {
	pathWithQuery := path
	if len(query) > 0 {
		pathWithQuery = path + "?" + query.Encode()
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := d.newRequest(ctx, method, path, query, body)
		if err != nil {
			return err
		}

		resp, err := d.config.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("hetznerhcloud: api request failed: %w", err)
		}

		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("hetznerhcloud: failed to read response: %w", readErr)
		}

		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			log.Warnf("hetznerhcloud: request %s %s failed with status %s (attempt %d/%d)", method, pathWithQuery, resp.Status, attempt, maxRetries)
			if attempt == maxRetries {
				return fmt.Errorf("hetznerhcloud: API request %s %s failed: %s", method, pathWithQuery, resp.Status)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			message := strings.TrimSpace(string(data))
			if message == "" {
				message = resp.Status
			}
			return fmt.Errorf("hetznerhcloud: API request %s %s failed: %s", method, pathWithQuery, message)
		}

		if into != nil && len(data) > 0 {
			if err := json.Unmarshal(data, into); err != nil {
				return fmt.Errorf("hetznerhcloud: decode response: %w", err)
			}
		}

		return nil
	}

	return nil
}

func (d *DNSProvider) newRequest(ctx context.Context, method, path string, query url.Values, body []byte) (*http.Request, error) {
	endpoint := d.baseURL.ResolveReference(&url.URL{Path: path})
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("hetznerhcloud: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+d.config.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func parseIdentifier(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("identifier missing")
	}

	trimmed := strings.Trim(string(raw), "\"")
	if trimmed == "" {
		return "", errors.New("identifier missing")
	}

	return trimmed, nil
}
