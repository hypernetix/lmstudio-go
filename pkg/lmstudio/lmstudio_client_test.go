package lmstudio

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockLogger is a simple mock implementation of the Logger interface for testing
type mockLogger struct {
	level    LogLevel
	messages []string
	mu       sync.Mutex
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		level:    LogLevelTrace,
		messages: make([]string, 0),
	}
}

func (l *mockLogger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *mockLogger) Error(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	l.messages = append(l.messages, msg)
	fmt.Println(msg)
}

func (l *mockLogger) Warn(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[WARN]  "+format, v...)
	l.messages = append(l.messages, msg)
	fmt.Println(msg)
}

func (l *mockLogger) Info(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[INFO]  "+format, v...)
	l.messages = append(l.messages, msg)
	fmt.Println(msg)
}

func (l *mockLogger) Debug(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[DEBUG] "+format, v...)
	l.messages = append(l.messages, msg)
	fmt.Println(msg)
}

func (l *mockLogger) Trace(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf("[TRACE] "+format, v...)
	l.messages = append(l.messages, msg)
	fmt.Println(msg)
}

func (l *mockLogger) getMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Return a copy to avoid race conditions
	result := make([]string, len(l.messages))
	copy(result, l.messages)
	return result
}

func (l *mockLogger) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = make([]string, 0)
}

// TestNewLMStudioClient tests the creation of a new LM Studio client
func TestNewLMStudioClient(t *testing.T) {
	// Test with default parameters
	client := NewLMStudioClient("", nil)
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.apiHost != fmt.Sprintf("http://%s:%d", LMStudioAPIHosts[0], LMStudioAPIPorts[0]) {
		t.Errorf("Expected default API host %s, got %s", fmt.Sprintf("http://%s:%d", LMStudioAPIHosts[0], LMStudioAPIPorts[0]), client.apiHost)
	}

	if client.logger == nil {
		t.Error("Expected non-nil default logger")
	}

	if client.connections == nil {
		t.Error("Expected non-nil connections map")
	}

	if client.ctx == nil {
		t.Error("Expected non-nil context")
	}

	if client.cancel == nil {
		t.Error("Expected non-nil cancel function")
	}

	// Test with custom parameters
	customHost := "localhost:5678"
	customLogger := newMockLogger()
	client = NewLMStudioClient(customHost, customLogger)

	if client.apiHost != customHost {
		t.Errorf("Expected custom API host %s, got %s", customHost, client.apiHost)
	}

	if client.logger != customLogger {
		t.Error("Expected custom logger")
	}
}

// TestLMStudioClientClose tests the Close method of the LM Studio client
func TestLMStudioClientClose(t *testing.T) {
	logger := newMockLogger()
	client := NewLMStudioClient("localhost:1234", logger)

	// Since we don't have actual connections, just verify that Close doesn't panic
	err := client.Close()
	if err != nil {
		t.Errorf("Expected no error from Close, got %v", err)
	}

	// Verify that the context is canceled
	select {
	case <-client.ctx.Done():
		// This is expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context was not canceled by Close")
	}
}

// TestListLoadedLLMs tests the ListLoadedLLMs method
func TestListLoadedLLMs(t *testing.T) {
	// Create a mock server that responds to the listLoaded endpoint
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Call the method we're testing
	models, err := client.ListLoadedLLMs()
	if err != nil {
		t.Fatalf("ListLoadedLLMs failed: %v", err)
	}

	// Verify the results
	if len(models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(models))
	}

	if models[0].ModelKey != "mock-model-7B" {
		t.Errorf("Unexpected model key: %s", models[0].ModelKey)
	}

	// Verify that the models are marked as loaded
	for i, model := range models {
		if !model.IsLoaded {
			t.Errorf("Expected model %d to be marked as loaded", i)
		}
	}
}

// TestListDownloadedModels tests the ListDownloadedModels method
func TestListDownloadedModels(t *testing.T) {
	// Create a mock server that responds to the listDownloadedModels endpoint
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Call the method we're testing
	models, err := client.ListDownloadedModels()
	if err != nil {
		t.Fatalf("ListDownloadedModels failed: %v", err)
	}

	// Verify the results
	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	if models[0].ModelKey != "mock-model-0.5B" || models[1].ModelKey != "mock-model-7B" {
		t.Errorf("Unexpected model keys: %s, %s", models[0].ModelKey, models[1].ModelKey)
	}

	// Verify that the models are not marked as loaded (since they're just downloaded)
	for i, model := range models {
		if model.IsLoaded {
			t.Errorf("Expected model %d to not be marked as loaded", i)
		}
	}
}

// TestUnloadModel tests the UnloadModel method
func TestUnloadModel(t *testing.T) {
	// Create a mock server that responds to the unloadModel endpoint
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Call the method we're testing
	err = client.UnloadModel("mock-model-0.5B")
	if err != nil {
		t.Fatalf("UnloadModel failed: %v", err)
	}
}

// TestLoadModel tests the LoadModel method
func TestLoadModel(t *testing.T) {
	fmt.Println("[TEST] TestLoadModel started")
	defer fmt.Println("[TEST] TestLoadModel finished or failed")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("TestLoadModel panicked: %v", r)
		}
	}()

	t.Parallel()
	// Add a timeout to prevent hanging forever
	done := make(chan struct{})
	go func() {
		// Create a mock server that handles model loading
		server := NewMockLMStudioService(t, newMockLogger())
		defer server.Close()

		// Extract the host and port from the server URL
		serverURL, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Failed to parse server URL: %v", err)
		}

		// Create a client that connects to our mock server
		logger := newMockLogger()
		client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
		defer client.Close()

		// Call the method we're testing
		err = client.LoadModel("mock-model-0.5B")
		if err != nil {
			t.Fatalf("LoadModel failed: %v", err)
		}
		close(done)

		// Verify that the logger recorded progress messages
		messages := logger.getMessages()
		if len(messages) == 0 {
			t.Errorf("Expected progress messages in logger, got none")
		}
	}()
	// Wait for test completion or timeout
	select {
	case <-done:
		// Test completed successfully
	case <-time.After(10 * time.Second):
		t.Fatal("TestLoadModel: test timed out (possible deadlock or missing mock response)")
	}
}

// TestSendPrompt tests the SendPrompt method
func TestSendPrompt(t *testing.T) {
	// Create a mock server that handles both model listing and chat
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	logger.SetLevel(LogLevelDebug)
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Collect tokens in a slice for verification
	receivedTokens := []string{}
	callback := func(token string) {
		logger.Debug("TestSendPrompt: Received token #%d: %s", len(receivedTokens)+1, token)
		receivedTokens = append(receivedTokens, token)
	}

	// Call the method we're testing
	err = client.SendPrompt("mock-model-0.5B", "What is the meaning of life?", 0.7, callback)
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}

	logger.Debug("Now compare received tokens with expected tokens")

	// Verify we received the expected tokens from the mock server
	expectedTokens := []string{"Hello", ", ", "world", "!"}
	if len(receivedTokens) != len(expectedTokens) {
		t.Fatalf("Expected %d tokens, got %d", len(expectedTokens), len(receivedTokens))
	}

	for i, token := range expectedTokens {
		if receivedTokens[i] != token {
			t.Errorf("Expected token %d to be '%s', got '%s'", i, token, receivedTokens[i])
		}
	}
}

// TestLoadModelWithProgress tests the LoadModelWithProgress method
func TestLoadModelWithProgress(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgress started")
	defer fmt.Println("[TEST] TestLoadModelWithProgress finished or failed")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("TestLoadModelWithProgress panicked: %v", r)
		}
	}()

	t.Parallel()
	// Add a timeout to prevent hanging forever
	done := make(chan struct{})
	go func() {
		// Create a mock server that handles model loading
		server := NewMockLMStudioService(t, newMockLogger())
		defer server.Close()

		// Extract the host and port from the server URL
		serverURL, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Failed to parse server URL: %v", err)
		}

		// Create a client that connects to our mock server
		logger := newMockLogger()
		client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
		defer client.Close()

		// Track progress callbacks
		var progressCallbacks []float64
		var modelInfoCallbacks []*Model
		var callbackMutex sync.Mutex

		progressCallback := func(progress float64, modelInfo *Model) {
			callbackMutex.Lock()
			defer callbackMutex.Unlock()
			progressCallbacks = append(progressCallbacks, progress)
			modelInfoCallbacks = append(modelInfoCallbacks, modelInfo)
			logger.Debug("Progress callback: %.2f%%, model: %v", progress*100, modelInfo != nil)
		}

		// Call the method
		err = client.LoadModelWithProgress(30*time.Second, "mock-model-0.5B", progressCallback)
		if err != nil {
			t.Fatalf("LoadModelWithProgress failed: %v", err)
		}

		// Wait a moment to ensure all progress callbacks are processed
		time.Sleep(100 * time.Millisecond)

		// Verify we got the expected number of progress callbacks
		callbackMutex.Lock()
		if len(progressCallbacks) == 0 {
			t.Errorf("Expected at least one progress callback, got %d", len(progressCallbacks))
		}

		// Check that progress values are in valid range
		for i, progress := range progressCallbacks {
			if progress < 0.0 || progress > 1.0 {
				t.Errorf("Progress callback %d has invalid value: %f (should be 0.0-1.0)", i, progress)
			}
		}

		// Check that the last progress value is 1.0 (completion)
		if len(progressCallbacks) > 0 && progressCallbacks[len(progressCallbacks)-1] != 1.0 {
			t.Errorf("Last progress value should be 1.0, got %f", progressCallbacks[len(progressCallbacks)-1])
		}
		callbackMutex.Unlock()

		close(done)
	}()

	// Wait for test completion or timeout
	select {
	case <-done:
		// Test completed successfully
	case <-time.After(10 * time.Second):
		t.Fatal("TestLoadModelWithProgress: test timed out (possible deadlock or missing mock response)")
	}
}

// TestLoadModelWithProgressAlreadyLoaded tests LoadModelWithProgress with an already loaded model
func TestLoadModelWithProgressAlreadyLoaded(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressAlreadyLoaded started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressAlreadyLoaded finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Track progress callbacks - for already loaded model we should get one callback with 100%
	var progressCallbacks []float64
	var callbackMutex sync.Mutex

	progressCallback := func(progress float64, modelInfo *Model) {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		progressCallbacks = append(progressCallbacks, progress)
		logger.Debug("Progress callback: %.2f%%", progress*100)
	}

	// Call with a model that is already loaded (mock-model-7B)
	err = client.LoadModelWithProgress(30*time.Second, "mock-model-7B", progressCallback)
	if err != nil {
		t.Fatalf("LoadModelWithProgress failed for already loaded model: %v", err)
	}

	// Verify that we got exactly one callback with 100% progress for already loaded model
	callbackMutex.Lock()
	if len(progressCallbacks) != 1 {
		t.Errorf("Expected exactly 1 progress callback for already loaded model, got %d", len(progressCallbacks))
	}

	if len(progressCallbacks) > 0 && progressCallbacks[0] != 1.0 {
		t.Errorf("Expected progress to be 1.0 for already loaded model, got %f", progressCallbacks[0])
	}
	callbackMutex.Unlock()
}

// TestLoadModelWithProgressNilCallback tests LoadModelWithProgress with nil callback
func TestLoadModelWithProgressNilCallback(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressNilCallback started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressNilCallback finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Call the method with nil callback (should not crash)
	err = client.LoadModelWithProgress(30*time.Second, "mock-model-0.5B", nil)
	if err != nil {
		t.Fatalf("LoadModelWithProgress with nil callback failed: %v", err)
	}
}

// TestLoadModelWithProgressContextCancellation tests LoadModelWithProgressContext with cancellation
func TestLoadModelWithProgressContextCancellation(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressContextCancellation started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressContextCancellation finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Track progress callbacks
	var progressCallbacks []float64
	var callbackMutex sync.Mutex

	progressCallback := func(progress float64, modelInfo *Model) {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		progressCallbacks = append(progressCallbacks, progress)
		logger.Debug("Progress callback before cancellation: %.2f%%", progress*100)

		// Cancel the context after receiving first progress update
		if len(progressCallbacks) == 1 {
			logger.Debug("Cancelling context after first progress update")
			cancel()
		}
	}

	// Call the method with cancellation context
	err = client.LoadModelWithProgressContext(ctx, 10*time.Second, "mock-model-0.5B", progressCallback)

	// Should receive a cancellation error
	if err == nil {
		t.Fatalf("Expected cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}

	// Verify we got at least one progress callback before cancellation
	callbackMutex.Lock()
	if len(progressCallbacks) == 0 {
		t.Errorf("Expected at least one progress callback before cancellation")
	}
	callbackMutex.Unlock()
}

// TestLoadModelWithProgressContextTimeout tests LoadModelWithProgressContext with timeout
func TestLoadModelWithProgressContextTimeout(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressContextTimeout started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressContextTimeout finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel it immediately

	// Call the method with cancelled context
	err = client.LoadModelWithProgressContext(ctx, 30*time.Second, "mock-model-0.5B", nil)

	// Should receive a cancellation error
	if err == nil {
		t.Fatalf("Expected cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}
}

// TestLoadModelWithProgressCancellation tests model load cancellation during loading
func TestLoadModelWithProgressCancellation(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressCancellation started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressCancellation finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Create a context that we'll cancel after first progress update
	ctx, cancel := context.WithCancel(context.Background())

	// Track progress callbacks
	var progressCallbacks []float64
	var callbackMutex sync.Mutex
	var cancelled bool

	progressCallback := func(progress float64, modelInfo *Model) {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		progressCallbacks = append(progressCallbacks, progress)
		logger.Debug("Progress callback: %.2f%%", progress*100)

		// Cancel after receiving the first progress update
		if len(progressCallbacks) == 1 && !cancelled {
			logger.Debug("Cancelling context after first progress update")
			cancelled = true
			cancel()
		}
	}

	// Call the method with cancellation context
	err = client.LoadModelWithProgressContext(ctx, 30*time.Second, "mock-model-0.5B", progressCallback)

	// Should receive a cancellation error
	if err == nil {
		t.Fatalf("Expected cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}

	// Verify we got at least one progress callback before cancellation
	callbackMutex.Lock()
	if len(progressCallbacks) == 0 {
		t.Errorf("Expected at least one progress callback before cancellation")
	}
	callbackMutex.Unlock()
}

// TestLoadModelWithProgressShortTimeout tests LoadModelWithProgress with a very short 1ms timeout
func TestLoadModelWithProgressShortTimeout(t *testing.T) {
	fmt.Println("[TEST] TestLoadModelWithProgressShortTimeout started")
	defer fmt.Println("[TEST] TestLoadModelWithProgressShortTimeout finished or failed")

	// Create a mock server
	server := NewMockLMStudioService(t, newMockLogger())
	defer server.Close()

	// Extract the host and port from the server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Create a client that connects to our mock server
	logger := newMockLogger()
	client := NewLMStudioClient(strings.TrimPrefix(serverURL.Host, "http://"), logger)
	defer client.Close()

	// Track progress callbacks
	var progressCallbacks []float64
	var callbackMutex sync.Mutex

	progressCallback := func(progress float64, modelInfo *Model) {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		progressCallbacks = append(progressCallbacks, progress)
		logger.Debug("Progress callback: %.2f%%", progress*100)
	}

	// Create a context that's already expired (deadline in the past)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()

	// Call the method with expired context - this should fail immediately
	err = client.LoadModelWithProgressContext(ctx, 30*time.Second, "mock-model-0.5B", progressCallback)

	// Should receive a cancellation error
	if err == nil {
		t.Fatalf("Expected timeout/cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout or cancellation error, got: %v", err)
	}

	// Verify that the operation was cancelled quickly - should have no progress callbacks
	callbackMutex.Lock()
	progressCount := len(progressCallbacks)
	callbackMutex.Unlock()

	logger.Debug("Received %d progress callbacks with expired context", progressCount)
	// With expired context, we should get 0 callbacks
	if progressCount > 0 {
		t.Errorf("Expected no progress callbacks due to expired context, got %d", progressCount)
	}
}
