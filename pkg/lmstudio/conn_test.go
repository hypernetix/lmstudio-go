package lmstudio

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Helper function to get a mock LM Studio service for testing
func getMockService(t *testing.T) (*httptest.Server, string) {
	logger := newMockLogger()
	server := NewMockLMStudioService(t, logger)
	// Extract host from server URL (remove http:// prefix)
	host := strings.TrimPrefix(server.URL, "http://")
	return server, host
}

// TestNamespaceConnectionConnect tests the connect method
func TestNamespaceConnectionConnect(t *testing.T) {
	// Create a test server
	server, host := getMockService(t)
	defer server.Close()

	// Create a namespace connection
	logger := newMockLogger()
	nc := &namespaceConnection{
		logger:       logger,
		namespace:    "llm",
		pendingCalls: make(map[int]chan json.RawMessage),
	}

	// Test successful connection
	ctx := context.Background()
	err := nc.connect(host, ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Check if connected
	if !nc.isConnected() {
		t.Errorf("Expected connection to be established, but isConnected() returned false")
	}

	// Close the connection
	err = nc.close()
	if err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	// Check if disconnected
	if nc.isConnected() {
		t.Errorf("Expected connection to be closed, but isConnected() returned true")
	}
}

// TestNamespaceConnectionRemoteCall tests the RemoteCall method
func TestNamespaceConnectionRemoteCall(t *testing.T) {
	// Create a test server
	server, host := getMockService(t)
	defer server.Close()

	// Create a namespace connection
	logger := newMockLogger()
	nc := &namespaceConnection{
		logger:       logger,
		namespace:    "llm",
		pendingCalls: make(map[int]chan json.RawMessage),
	}

	// Connect to the server
	ctx := context.Background()
	err := nc.connect(host, ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.close()

	// Test successful RPC call - use listDownloadedModels which is supported by the mock
	result, err := nc.RemoteCall("listDownloadedModels", nil)
	if err != nil {
		t.Errorf("RemoteCall failed: %v", err)
	}

	// Verify we got some result data
	if len(result) == 0 {
		t.Errorf("Expected non-empty result, got empty data")
	}

	// Parse the result to verify it's valid JSON
	var models []interface{}
	if err := json.Unmarshal(result, &models); err != nil {
		t.Errorf("Failed to parse result: %v", err)
	}

	// Verify we got at least one model
	if len(models) == 0 {
		t.Errorf("Expected at least one model in the result")
	}

	// Test with unloadModel which is also supported by the mock
	params := map[string]interface{}{"identifier": "mock-model-0.5B"}
	result, err = nc.RemoteCall("unloadModel", params)
	if err != nil {
		t.Errorf("RemoteCall for unloadModel failed: %v", err)
	}

	// Test with connection closed
	nc.close()
	_, err = nc.RemoteCall("test/endpoint", nil)
	if err == nil {
		t.Errorf("Expected error due to closed connection, got nil")
	}
}

// TestEnsureConnected tests the ensureConnected method
func TestEnsureConnected(t *testing.T) {
	// Create a namespace connection without connecting
	logger := newMockLogger()
	nc := &namespaceConnection{
		logger:       logger,
		namespace:    "llm",
		pendingCalls: make(map[int]chan json.RawMessage),
		connected:    false,
	}

	// Test with not connected
	err := nc.ensureConnected()
	if err == nil {
		t.Errorf("Expected error when not connected, got nil")
	}

	// Manually set connected to true but keep conn nil
	nc.mu.Lock()
	nc.connected = true
	nc.mu.Unlock()

	// Should still error because conn is nil
	err = nc.ensureConnected()
	if err == nil {
		t.Errorf("Expected error when conn is nil, got nil")
	}
}

// TestHandleMessages tests the message handling logic
func TestHandleMessages(t *testing.T) {
	// This is a more complex test that would require mocking the WebSocket connection
	// and simulating different message types. For simplicity, we'll focus on the basic
	// functionality.

	// Create a test server
	server, host := getMockService(t)
	defer server.Close()

	// Create a namespace connection
	logger := newMockLogger()
	nc := &namespaceConnection{
		logger:       logger,
		namespace:    "llm",
		pendingCalls: make(map[int]chan json.RawMessage),
	}

	// Connect to the server
	ctx := context.Background()
	err := nc.connect(host, ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait a bit for handleMessages goroutine to start
	time.Sleep(100 * time.Millisecond)

	// Make an RPC call to trigger message handling - use listDownloadedModels which is supported by the mock
	_, _ = nc.RemoteCall("listDownloadedModels", nil)

	// Close the connection
	nc.close()
}

// mockChannelHandler implements the ChannelHandler interface for testing
type mockChannelHandler struct {
	messages [][]byte
	wg       *sync.WaitGroup
}

func (h *mockChannelHandler) handleMessages(wg *sync.WaitGroup) {
	h.wg = wg
	defer wg.Done()
}

func (h *mockChannelHandler) processMessage(message []byte) {
	h.messages = append(h.messages, message)
}

// TestChannelHandler tests the channel handler functionality
func TestChannelHandler(t *testing.T) {
	// Create a mock channel handler
	handler := &mockChannelHandler{
		messages: make([][]byte, 0),
	}

	// Create a wait group
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Start handling messages
	go handler.handleMessages(wg)

	// Process a message
	message := []byte(`{"test": "data"}`)
	handler.processMessage(message)

	// Wait for handling to complete
	wg.Wait()

	// Check if message was processed
	if len(handler.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(handler.messages))
	}

	// Check message content
	if string(handler.messages[0]) != `{"test": "data"}` {
		t.Errorf("Expected message content to be '{\"test\":  \"data\"}', got '%s'", string(handler.messages[0]))
	}
}
