package lmstudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// LMStudioClient represents a client for LM Studio service
type LMStudioClient struct {
	logger      Logger
	apiHost     string
	connections map[string]*namespaceConnection
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewLMStudioClient creates a new LM Studio client
func NewLMStudioClient(apiHost string, logger Logger) *LMStudioClient {
	if apiHost == "" {
		apiHost = fmt.Sprintf("http://%s:%d", LMStudioAPIHosts[0], LMStudioAPIPorts[0])
	}

	if logger == nil {
		logger = NewLogger(LogLevelError)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &LMStudioClient{
		logger:      logger,
		apiHost:     apiHost,
		connections: make(map[string]*namespaceConnection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// getConnection gets or creates a connection to a specific namespace
func (c *LMStudioClient) getConnection(namespace string) (*namespaceConnection, error) {
	c.mu.Lock()
	conn, exists := c.connections[namespace]
	if exists && conn.connected {
		c.mu.Unlock()
		return conn, nil
	}

	// Create a new connection if it doesn't exist or is disconnected
	conn = &namespaceConnection{
		logger:         c.logger,
		namespace:      namespace,
		nextID:         1,
		pendingCalls:   make(map[int]chan json.RawMessage),
		activeChannels: make(map[int]ChannelHandler),
		connected:      false,
	}
	c.connections[namespace] = conn
	c.mu.Unlock()

	// Connect to the namespace
	if err := conn.connect(c.apiHost, c.ctx); err != nil {
		return nil, err
	}

	return conn, nil
}

// Close closes all namespace connections
func (c *LMStudioClient) Close() error {
	// Cancel the context to stop all message handlers
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error
	for namespace, conn := range c.connections {
		if err := conn.close(); err != nil {
			lastErr = fmt.Errorf("failed to close %s connection: %w", namespace, err)
			c.logger.Error("Error closing %s connection: %v", namespace, err)
		}
	}

	return lastErr
}

// ListLoadedLLMs lists all loaded models available in LM Studio
func (c *LMStudioClient) ListLoadedLLMs() ([]Model, error) {
	// Get or create a connection to the LLM namespace
	conn, err := c.getConnection(LLMNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LLM namespace: %w", err)
	}

	// Add a bit more logging to track request/response flow
	c.logger.Debug("Sending listLoaded request to LLM namespace")

	// Make the RPC call to list loaded models
	result, err := conn.RemoteCall(ModelListLoadedEndpoint, nil)
	if err != nil {
		return nil, err
	}

	// Debug the response
	c.logger.Debug("Raw LLM loaded models response: %s", string(result))

	// Parse the response into the unified Model struct
	var models []Model
	if err := json.Unmarshal(result, &models); err != nil {
		return nil, fmt.Errorf("failed to parse loaded models response: %w", err)
	} else {
		// Mark all models as loaded
		for i := range models {
			models[i].IsLoaded = true
		}
	}

	return models, nil
}

// ListDownloadedModels lists all downloaded models available in LM Studio
func (c *LMStudioClient) ListDownloadedModels() ([]Model, error) {
	// Get or create a connection to the system namespace
	conn, err := c.getConnection(SystemAPINamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system namespace: %w", err)
	}

	// Add a bit more logging to track request/response flow
	c.logger.Debug("Sending listDownloadedModels request to system namespace")

	// Make the RPC call to list downloaded models
	result, err := conn.RemoteCall(ModelListDownloadedEndpoint, nil)
	if err != nil {
		return nil, err
	}

	// Debug the response
	c.logger.Debug("Raw system downloaded models response: %s", string(result))

	// Parse the response into the unified Model struct
	var models []Model
	if err := json.Unmarshal(result, &models); err != nil {
		return nil, fmt.Errorf("failed to parse downloaded models response: %w", err)
	}

	return models, nil
}

// NewModelLoadingChannel creates a new channel for loading a model
func (c *LMStudioClient) NewModelLoadingChannel(namespace string, progressFn func(float64)) (*ModelLoadingChannel, error) {
	conn, err := c.getConnection(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s namespace: %w", namespace, err)
	}

	return &ModelLoadingChannel{
		namespace:    namespace,
		conn:         conn,
		channelID:    0, // Will be assigned when channel is created
		modelKey:     "",
		progressFn:   progressFn,
		resultCh:     make(chan ModelLoadingResult, 1),
		errorCh:      make(chan error, 1),
		cancelCh:     make(chan struct{}),
		messageCh:    make(chan []byte, 10), // Buffer for incoming messages
		isFinished:   false,
		lastProgress: -1.0,
	}, nil
}

// checkModelExists verifies if the model exists in the downloaded models
func (c *LMStudioClient) checkModelExists(modelIdentifier string) error {
	downloaded, err := c.ListDownloadedModels()
	if err != nil {
		return fmt.Errorf("failed to check downloaded models: %w", err)
	}

	for _, model := range downloaded {
		if model.ModelKey == modelIdentifier {
			return nil
		}
	}

	return fmt.Errorf("model %s not found in downloaded models", modelIdentifier)
}

// isModelAlreadyLoaded checks if the model is already loaded
func (c *LMStudioClient) isModelAlreadyLoaded(modelIdentifier string) bool {
	loaded, err := c.ListLoadedLLMs()
	if err != nil {
		c.logger.Warn("Warning: Failed to check if model is already loaded: %v", err)
		return false
	}

	for _, model := range loaded {
		if model.Identifier == modelIdentifier || model.ModelKey == modelIdentifier {
			c.logger.Debug("Model %s is already loaded", modelIdentifier)
			return true
		}
	}
	return false
}

// waitForModelLoading waits for a model to finish loading
func (c *LMStudioClient) waitForModelLoading(channel *ModelLoadingChannel, modelIdentifier string, loadTimeout time.Duration) error {
	c.logger.Debug("Waiting for model %s to load (timeout: %d seconds)...",
		modelIdentifier, int(loadTimeout.Seconds()))

	// Set up a separate timeout for logging purposes
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()

	// Use a context with timeout for proper cancellation
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	defer cancel()

	// Set up channels for response handling
	resultCh := make(chan *ModelLoadingResult, 1)
	errCh := make(chan error, 1)

	// Start goroutine to wait for result
	go func() {
		result, err := channel.WaitForResult(loadTimeout)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	// Wait for either a result, error, or timeout
	for {
		select {
		case result := <-resultCh:
			if !result.Success {
				return fmt.Errorf("model %s failed to load", modelIdentifier)
			}
			c.logger.Debug("Model %s loaded successfully with identifier: %s", modelIdentifier, result.Identifier)
			return nil

		case err := <-errCh:
			return fmt.Errorf("model loading failed: %w", err)

		case <-logTicker.C:
			c.logger.Debug("Still waiting for model %s to load... (channel ID: %d)",
				modelIdentifier, channel.channelID)

		case <-ctx.Done():
			return fmt.Errorf("model loading timed out after %v", loadTimeout)
		}
	}
}

// LoadModel loads a specified model in LM Studio
func (c *LMStudioClient) LoadModel(modelIdentifier string) error {
	// Check if the model exists in downloaded models
	if err := c.checkModelExists(modelIdentifier); err != nil {
		return err
	}

	// Check if the model is already loaded
	if c.isModelAlreadyLoaded(modelIdentifier) {
		return nil
	}

	// Create a model loading channel
	c.logger.Debug("Creating model loading channel for: %s", modelIdentifier)
	channel, err := c.NewModelLoadingChannel(LLMNamespace, func(progress float64) {
		c.logger.Debug("Loading model %s: %.1f%% complete", modelIdentifier, progress*100)
	})
	if err != nil {
		return fmt.Errorf("failed to create model loading channel: %w", err)
	}
	defer channel.Close()

	// Create the channel and start loading the model
	err = channel.CreateChannel(modelIdentifier)
	if err != nil {
		return fmt.Errorf("failed to start model loading: %w", err)
	}

	// Use a longer timeout for model loading - some large models can take several minutes
	loadTimeout := 120 * time.Second
	return c.waitForModelLoading(channel, modelIdentifier, loadTimeout)
}

// UnloadModel unloads a specified model in LM Studio
func (c *LMStudioClient) UnloadModel(modelIdentifier string) error {
	// Get or create a connection to the LLM namespace
	conn, err := c.getConnection(LLMNamespace)
	if err != nil {
		return fmt.Errorf("failed to connect to LLM namespace: %w", err)
	}

	// Construct the parameters for unloading a model
	params := map[string]interface{}{
		"identifier": modelIdentifier,
	}

	c.logger.Debug("Sending unloadModel request for model: %s", modelIdentifier)

	// Make the RPC call to unload the model
	_, err = conn.RemoteCall(ModelUnloadEndpoint, params)
	if err != nil {
		return err
	}

	// Only log success if we didn't get an error
	c.logger.Debug("Successfully unloaded model: %s", modelIdentifier)
	return nil
}

// UnloadAllModels unloads all currently loaded models in LM Studio
func (c *LMStudioClient) UnloadAllModels() error {
	// Get the list of all loaded models
	loadedModels, err := c.ListAllLoadedModels()
	if err != nil {
		return fmt.Errorf("failed to list loaded models: %w", err)
	}

	if len(loadedModels) == 0 {
		c.logger.Debug("No models are currently loaded")
		return nil
	}

	c.logger.Debug("Unloading all %d loaded models", len(loadedModels))

	// Unload each model
	for _, model := range loadedModels {
		identifier := model.Identifier
		if identifier == "" {
			identifier = model.ModelKey
		}

		c.logger.Debug("Unloading model: %s", identifier)
		err := c.UnloadModel(identifier)
		if err != nil {
			c.logger.Warn("Failed to unload model %s: %v", identifier, err)
			// Continue with other models even if one fails
		}
	}

	c.logger.Debug("Successfully unloaded all models")
	return nil
}

// ListLoadedEmbeddingModels lists all loaded embedding models available in LM Studio
func (c *LMStudioClient) ListLoadedEmbeddingModels() ([]Model, error) {
	// Get or create a connection to the embedding namespace
	conn, err := c.getConnection(EmbeddingNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to embedding namespace: %w", err)
	}

	// Add a bit more logging to track request/response flow
	c.logger.Debug("Sending listLoaded request to embedding namespace")

	// Make the RPC call to list loaded embedding models
	result, err := conn.RemoteCall(ModelListLoadedEndpoint, nil)
	if err != nil {
		return nil, err
	}

	// Debug the response
	c.logger.Debug("Raw embedding loaded models response: %s", string(result))

	// Parse the response into the unified Model struct
	var models []Model
	if err := json.Unmarshal(result, &models); err != nil {
		return nil, fmt.Errorf("failed to parse loaded embedding models response: %w", err)
	} else {
		// Mark all models as loaded
		for i := range models {
			models[i].IsLoaded = true
		}
	}

	return models, nil
}

// ListAllLoadedModels lists all loaded models (both LLM and embedding) available in LM Studio
func (c *LMStudioClient) ListAllLoadedModels() ([]Model, error) {
	var allModels []Model

	// Get loaded LLM models
	llmModels, err := c.ListLoadedLLMs()
	if err != nil {
		c.logger.Debug("Warning: Failed to list loaded LLM models: %v", err)
	} else {
		// Add model category to differentiate in the combined list
		for i := range llmModels {
			if llmModels[i].Type == "" {
				llmModels[i].Type = "llm"
			}
		}
		allModels = append(allModels, llmModels...)
	}

	// Get loaded embedding models
	embeddingModels, err := c.ListLoadedEmbeddingModels()
	if err != nil {
		c.logger.Debug("Warning: Failed to list loaded embedding models: %v", err)
	} else {
		// Add model category to differentiate in the combined list
		for i := range embeddingModels {
			if embeddingModels[i].Type == "" {
				embeddingModels[i].Type = "embedding"
			}
		}
		allModels = append(allModels, embeddingModels...)
	}

	return allModels, nil
}

// CheckStatus checks if the LM Studio service is running and accessible
func (c *LMStudioClient) CheckStatus() (bool, error) {
	// Try to connect to the system namespace as a way to check if the service is running
	conn, err := c.getConnection(SystemAPINamespace)
	if err != nil {
		return false, nil // Service is not running or not accessible
	}

	// If we can connect, check if we can make a simple API call
	_, err = conn.RemoteCall(ModelListDownloadedEndpoint, nil)
	if err != nil {
		return false, errors.New("service is running but API is not responding correctly: " + err.Error())
	}

	// If we get here, the service is running and responding to API calls
	return true, nil
}

// SendPrompt sends a prompt to the model and streams back the response
func (c *LMStudioClient) SendPrompt(modelIdentifier string, prompt string, temperature float64, callback func(token string)) error {
	// First, check if the model is loaded
	loaded, err := c.ListLoadedLLMs()
	if err != nil {
		return fmt.Errorf("failed to check if model is loaded: %w", err)
	}

	var instanceReference string
	modelLoaded := false
	for _, model := range loaded {
		if model.Identifier == modelIdentifier || model.ModelKey == modelIdentifier {
			modelLoaded = true
			// Use the exact identifier from the loaded model
			modelIdentifier = model.Identifier
			instanceReference = model.InstanceReference
			break
		}
	}

	if !modelLoaded {
		// Try to load the model
		c.logger.Debug("Model %s is not loaded. Attempting to load it now...", modelIdentifier)
		if err := c.LoadModel(modelIdentifier); err != nil {
			return fmt.Errorf("failed to load model %s: %w", modelIdentifier, err)
		}
		c.logger.Debug("Model %s loaded successfully", modelIdentifier)

		// Get the instance reference after loading
		loaded, err := c.ListLoadedLLMs()
		if err != nil {
			return fmt.Errorf("failed to get loaded model details: %w", err)
		}

		for _, model := range loaded {
			if model.Identifier == modelIdentifier || model.ModelKey == modelIdentifier {
				instanceReference = model.InstanceReference
				break
			}
		}
	}

	if instanceReference == "" {
		return fmt.Errorf("could not find instance reference for model %s", modelIdentifier)
	}

	// Connect to the LLM namespace
	conn, err := c.getConnection(LLMNamespace)
	if err != nil {
		return fmt.Errorf("failed to connect to LLM namespace: %w", err)
	}

	// Create a channel to receive streaming responses
	streamCh := make(chan string, 100)
	errCh := make(chan error, 1)
	doneCh := make(chan struct{})

	// Create a unique channel ID for this chat session
	chatChannelID := rand.Intn(100000) + 1

	// Create and register the streaming channel
	streamingChannel := NewModelStreamingChannel(chatChannelID, conn, streamCh, errCh, doneCh)

	// Register the streaming channel with the namespace connection
	conn.mu.Lock()
	conn.activeChannels[chatChannelID] = streamingChannel
	conn.mu.Unlock()

	// Create the chat message using the corrected structure based on the server's error messages
	chatMsg := map[string]interface{}{
		"type":      "channelCreate",
		"channelId": chatChannelID,
		"endpoint":  ModelChatEndpoint, // Using "predict" endpoint
		"creationParameter": map[string]interface{}{
			"modelSpecifier": map[string]interface{}{
				"type":              "instanceReference",
				"instanceReference": instanceReference,
			},
			"history": map[string]interface{}{
				"messages": []map[string]interface{}{
					{
						"role": "user",
						"content": []map[string]interface{}{{ // Content must be an array of objects
							"type": "text",
							"text": prompt,
						}},
					},
				},
			},
			"predictionConfigStack": map[string]interface{}{
				"layers": []interface{}{
					map[string]interface{}{
						"layerName": "instance", // Use a valid enum value from the error list
						"config": map[string]interface{}{
							"temperature": temperature,
							"maxTokens":   4096,
							"stream":      true,
							"fields":      []interface{}{}, // Changed from object to array as required
						},
					},
				},
			},
		},
	}

	// Send the chat message
	c.logger.Debug("Sending prompt to model %s using predict endpoint (instance: %s, temp: %.2f)", modelIdentifier, instanceReference, temperature)
	if err := conn.conn.WriteJSON(chatMsg); err != nil {
		// Clean up
		conn.mu.Lock()
		delete(conn.activeChannels, chatChannelID)
		conn.mu.Unlock()
		return fmt.Errorf("failed to send chat message: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start goroutine to handle messages
	go streamingChannel.handleMessages(&wg)

	tokenCount := 0

	// Start a goroutine to process the streaming response
	go func() {
		defer wg.Done()
		c.logger.Debug("Starting token processing goroutine")
		for {
			select {
			case token := <-streamCh:
				// Call the callback with the token
				tokenCount++
				c.logger.Debug("Received token #%d from stream: %s", tokenCount, token)
				callback(token)
				c.logger.Debug("Called callback for token #%d", tokenCount)
			case err := <-errCh:
				// Print the error and exit
				c.logger.Error("Error during chat: %v", err)
				close(doneCh)
				return
			case <-doneCh:
				// Clean up
				// Check if there are any remaining tokens in the channel
				// and drain them before closing
				for {
					select {
					case token, ok := <-streamCh:
						if !ok {
							break
						}
						tokenCount++
						c.logger.Debug("Received final token #%d from stream: %s", tokenCount, token)
						callback(token)
						c.logger.Debug("Called callback for final token #%d", tokenCount)
					default:
						// No more tokens in the channel
						c.logger.Debug("No more tokens in stream channel")
						return
					}
				}

				c.logger.Debug("Token processing goroutine ending, processed %d tokens", tokenCount)
				conn.mu.Lock()
				delete(conn.activeChannels, chatChannelID)
				conn.mu.Unlock()
				return
			}
		}
	}()

	// Wait for the token processing goroutine to complete
	c.logger.Debug("Waiting for token processing to complete...")
	wg.Wait()
	c.logger.Debug("Chat completed, processed %d tokens", tokenCount)

	return nil
}
