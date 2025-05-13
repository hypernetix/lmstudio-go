package lmstudio

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateUrls(t *testing.T) {
	tests := []struct {
		name     string
		hosts    []string
		ports    []int
		expected []string
	}{
		{
			name:  "empty inputs should use defaults",
			hosts: []string{},
			ports: []int{1234},
			expected: []string{
				"http://localhost:1234",
				"https://localhost:1234",
				"http://127.0.0.1:1234",
				"https://127.0.0.1:1234",
				"http://0.0.0.0:1234",
				"https://0.0.0.0:1234",
			},
		},
		{
			name:  "custom hosts and ports",
			hosts: []string{"test.com", "example.com"},
			ports: []int{8080, 9090},
			expected: []string{
				"http://test.com:8080",
				"https://test.com:8080",
				"http://test.com:9090",
				"https://test.com:9090",
				"http://example.com:8080",
				"https://example.com:8080",
				"http://example.com:9090",
				"https://example.com:9090",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := generateUrls(tt.hosts, tt.ports)
			if len(urls) != len(tt.expected) {
				t.Errorf("generateUrls() returned %d urls: (%v), want %d (%v)", len(urls), urls, len(tt.expected), tt.expected)
			}

			// Create a map for easier comparison
			urlMap := make(map[string]bool)
			for _, url := range urls {
				urlMap[url] = true
			}

			for _, expected := range tt.expected {
				if !urlMap[expected] {
					t.Errorf("generateUrls() missing expected URL: %s", expected)
				}
			}
		})
	}
}

func TestDiscoverLMStudioServer(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tests := []struct {
		name        string
		host        string
		port        int
		serverURL   string
		expectError bool
	}{
		{
			name:        "server found",
			host:        "localhost",
			port:        1234,
			serverURL:   server.URL,
			expectError: false,
		},
		{
			name:        "server not found",
			host:        "nonexistent",
			port:        9999,
			serverURL:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(LogLevelDebug)
			url, err := DiscoverLMStudioServer(tt.host, tt.port, logger)

			if tt.expectError {
				if err == nil {
					t.Error("DiscoverLMStudioServer() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("DiscoverLMStudioServer() unexpected error: %v", err)
				}
				if url == "" {
					t.Error("DiscoverLMStudioServer() returned empty URL")
				}
			}
		})
	}
}

func TestIsServerRunning(t *testing.T) {
	// Create a test server that returns 200 OK
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer goodServer.Close()

	// Create a test server that returns 404 Not Found
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer badServer.Close()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "server running",
			url:      goodServer.URL,
			expected: true,
		},
		{
			name:     "server not running",
			url:      badServer.URL,
			expected: false,
		},
		{
			name:     "invalid URL",
			url:      "http://invalid-url-that-does-not-exist",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(LogLevelDebug)
			result := isServerRunning(tt.url, logger)
			if result != tt.expected {
				t.Errorf("isServerRunning() = %v, want %v", result, tt.expected)
			}
		})
	}
}
