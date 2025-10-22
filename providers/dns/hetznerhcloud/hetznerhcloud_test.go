package hetznerhcloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-acme/lego/v4/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *recordingLogger) Fatal(args ...any)                 { l.Printf("%v", args...) }
func (l *recordingLogger) Fatalln(args ...any)               { l.Printf("%v", args...) }
func (l *recordingLogger) Fatalf(format string, args ...any) { l.Printf(format, args...) }
func (l *recordingLogger) Print(args ...any)                 { l.Printf("%v", args...) }
func (l *recordingLogger) Println(args ...any)               { l.Printf("%v", args...) }
func (l *recordingLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *recordingLogger) containsSubstring(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, msg := range l.messages {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

func TestDNSProvider_PresentAndCleanUp_Success(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "example.com", r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"zones": []map[string]any{{
				"id":   123,
				"name": "example.com",
			}},
			"meta": map[string]any{
				"pagination": map[string]any{
					"next_page": nil,
				},
			},
		}))
	})

	mux.HandleFunc("/v1/zones/123/records", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "_acme-challenge", payload["name"])
		assert.Equal(t, "TXT", payload["type"])

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"record": map[string]any{
				"id":    "456",
				"name":  payload["name"],
				"type":  payload["type"],
				"value": payload["value"],
			},
		}))
	})

	mux.HandleFunc("/v1/zones/123/records/456", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	config := NewDefaultConfig()
	config.BaseURL = server.URL
	config.Token = "secret"
	config.HTTPClient = server.Client()

	provider, err := NewDNSProviderConfig(config)
	require.NoError(t, err)

	provider.findZoneByFqdn = func(string) (string, error) {
		return "example.com.", nil
	}

	err = provider.Present("example.com", "token", "keyAuth")
	require.NoError(t, err)

	err = provider.CleanUp("example.com", "token", "keyAuth")
	require.NoError(t, err)
}

func TestDNSProvider_PresentAndCleanUp_SuccessWithMixedCaseZone(t *testing.T) {
	mux := http.NewServeMux()

	zoneRequests := 0

	mux.HandleFunc("/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		zoneRequests++
		require.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Example.com", r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"zones": []map[string]any{{
				"id":   "123",
				"name": "example.com",
			}},
			"meta": map[string]any{
				"pagination": map[string]any{
					"next_page": nil,
				},
			},
		}))
	})

	mux.HandleFunc("/v1/zones/123/records", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "_acme-challenge", payload["name"])
		assert.Equal(t, "TXT", payload["type"])

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"record": map[string]any{
				"id":    "456",
				"name":  payload["name"],
				"type":  payload["type"],
				"value": payload["value"],
			},
		}))
	})

	mux.HandleFunc("/v1/zones/123/records/456", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	config := NewDefaultConfig()
	config.BaseURL = server.URL
	config.Token = "secret"
	config.HTTPClient = server.Client()

	provider, err := NewDNSProviderConfig(config)
	require.NoError(t, err)

	provider.findZoneByFqdn = func(string) (string, error) {
		return "Example.com.", nil
	}

	err = provider.Present("example.com", "token", "keyAuth")
	require.NoError(t, err)

	err = provider.CleanUp("example.com", "token", "keyAuth")
	require.NoError(t, err)

	assert.Equal(t, 1, zoneRequests)
}

func TestDNSProvider_Present_ZoneNotFound(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"zones": []map[string]any{},
			"meta": map[string]any{
				"pagination": map[string]any{
					"next_page": nil,
				},
			},
		}))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	config := NewDefaultConfig()
	config.BaseURL = server.URL
	config.Token = "secret"
	config.HTTPClient = server.Client()

	provider, err := NewDNSProviderConfig(config)
	require.NoError(t, err)

	provider.findZoneByFqdn = func(string) (string, error) {
		return "example.com.", nil
	}

	err = provider.Present("example.com", "token", "keyAuth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zone \"example.com\" not found")
}

func TestDNSProvider_Present_RecordCreationFailure(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"zones": []map[string]any{{
				"id":   123,
				"name": "example.com",
			}},
			"meta": map[string]any{
				"pagination": map[string]any{
					"next_page": nil,
				},
			},
		}))
	})

	var recordRequests int

	mux.HandleFunc("/v1/zones/123/records", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		recordRequests++
		http.Error(w, "error", http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	backupLogger := log.Logger
	logger := &recordingLogger{}
	log.Logger = logger
	t.Cleanup(func() {
		log.Logger = backupLogger
	})

	config := NewDefaultConfig()
	config.BaseURL = server.URL
	config.Token = "secret"
	config.HTTPClient = server.Client()

	provider, err := NewDNSProviderConfig(config)
	require.NoError(t, err)

	provider.findZoneByFqdn = func(string) (string, error) {
		return "example.com.", nil
	}

	err = provider.Present("example.com", "token", "keyAuth")
	require.Error(t, err)
	assert.Equal(t, 3, recordRequests)
	assert.True(t, logger.containsSubstring("[WARN]"))
}

func TestDNSProvider_CleanUp_RecordDeletionFailure(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"zones": []map[string]any{{
				"id":   123,
				"name": "example.com",
			}},
			"meta": map[string]any{
				"pagination": map[string]any{
					"next_page": nil,
				},
			},
		}))
	})

	mux.HandleFunc("/v1/zones/123/records", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"record": map[string]any{
				"id":   "456",
				"name": "_acme-challenge",
				"type": "TXT",
			},
		}))
	})

	var deleteRequests int

	mux.HandleFunc("/v1/zones/123/records/456", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		deleteRequests++
		http.Error(w, "error", http.StatusBadGateway)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	backupLogger := log.Logger
	logger := &recordingLogger{}
	log.Logger = logger
	t.Cleanup(func() {
		log.Logger = backupLogger
	})

	config := NewDefaultConfig()
	config.BaseURL = server.URL
	config.Token = "secret"
	config.HTTPClient = server.Client()

	provider, err := NewDNSProviderConfig(config)
	require.NoError(t, err)

	provider.findZoneByFqdn = func(string) (string, error) {
		return "example.com.", nil
	}

	require.NoError(t, provider.Present("example.com", "token", "keyAuth"))

	err = provider.CleanUp("example.com", "token", "keyAuth")
	require.Error(t, err)
	assert.Equal(t, 3, deleteRequests)
	assert.True(t, logger.containsSubstring("[WARN]"))
}
