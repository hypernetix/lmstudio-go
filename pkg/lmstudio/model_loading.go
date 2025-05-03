package lmstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type ModelLoadingResult struct {
	Identifier string `json:"identifier"`
	Success    bool   `json:"success"`
}

// ModelLoadingChannel handles the channel-based loading of models
type ModelLoadingChannel struct {
	namespace    string
	conn         *namespaceConnection
	channelID    int
	modelKey     string // Store the model key for debugging
	progressFn   func(float64)
	resultCh     chan ModelLoadingResult
	errorCh      chan error
	cancelCh     chan struct{}
	messageCh    chan []byte // Channel to receive messages from the main handler
	isFinished   bool
	lastProgress float64
	mu           sync.Mutex
}

// CreateChannel sends a channelCreate message to create a model loading channel
func (ch *ModelLoadingChannel) CreateChannel(modelKey string) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Store model key for debugging
	ch.modelKey = modelKey

	// Generate a unique channel ID
	ch.channelID = rand.Intn(100000) + 1

	// Register this channel with the namespace connection
	ch.conn.mu.Lock()
	ch.conn.activeChannels[ch.channelID] = ch
	ch.conn.mu.Unlock()

	// Create channel creation message - match Python SDK format exactly
	createMsg := map[string]interface{}{
		"type":      "channelCreate",
		"channelId": ch.channelID,
		"endpoint":  "loadModel",
		"creationParameter": map[string]interface{}{
			"modelKey":   modelKey,
			"identifier": modelKey, // Use the model key as identifier (not null)
			"loadConfigStack": map[string]interface{}{
				"layers": []interface{}{}, // Add required 'layers' property as empty array
			},
		},
	}

	ch.conn.logger.Debug("Creating model loading channel for model: %s with channelId: %d",
		modelKey, ch.channelID)

	// Clear any read deadline
	ch.conn.mu.Lock()
	ch.conn.conn.SetReadDeadline(time.Time{})
	ch.conn.mu.Unlock()

	// Send the channel create message
	err := ch.conn.conn.WriteJSON(createMsg)
	if err != nil {
		// Clean up on error
		ch.conn.mu.Lock()
		delete(ch.conn.activeChannels, ch.channelID)
		ch.conn.mu.Unlock()
		return fmt.Errorf("failed to create model loading channel: %w", err)
	}

	// Start goroutine to handle messages for this channel
	go ch.handleMessages(nil)

	return nil
}

// handleMessages processes messages for this channel
func (ch *ModelLoadingChannel) handleMessages(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	ch.conn.logger.Debug("Started message handler for channel %d (model: %s)", ch.channelID, ch.modelKey)

	for {
		// Check if channel has been closed
		select {
		case <-ch.cancelCh:
			ch.conn.logger.Debug("Channel message handler for %d stopped (cancel signal received)", ch.channelID)
			return
		case message := <-ch.messageCh:
			// Process message forwarded from the namespace handler
			ch.processMessage(message)
		case <-time.After(5 * time.Second):
			// Log a heartbeat every 5 seconds
			ch.conn.logger.Debug("Channel %d (model: %s) waiting for messages... (%.1f%% done)",
				ch.channelID, ch.modelKey, ch.lastProgress*100)

			// Check connection status
			ch.conn.mu.Lock()
			isConnected := ch.conn.connected
			ch.conn.mu.Unlock()

			if !isConnected {
				ch.conn.logger.Debug("Connection lost while waiting for model to load")
				ch.errorCh <- fmt.Errorf("connection lost while waiting for model to load")
				return
			}
		}
	}
}

// processMessage processes a message received from the namespace handler
func (ch *ModelLoadingChannel) processMessage(message []byte) {
	// Log raw message for debugging
	ch.conn.logger.Trace("================================================")
	ch.conn.logger.Trace("Channel %d received message: %s", ch.channelID, string(message))

	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		ch.conn.logger.Error("Error parsing message on channel %d: %v", ch.channelID, err)
		return
	}

	// Debug: Log the message with pretty formatting
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, message, "", "  "); err == nil {
		ch.conn.logger.Trace("Channel %d pretty message: \n%s", ch.channelID, prettyJSON.String())
	}

	// Get message type
	msgType, ok := msg["type"].(string)
	if !ok {
		ch.conn.logger.Error("Channel %d: Message missing type field", ch.channelID)
		return // Ignore messages without a type
	}

	ch.conn.logger.Trace("Processing %s message for channel %d", msgType, ch.channelID)

	switch msgType {
	case "channelSend":
		// Process channel messages according to Python SDK format
		if messageContent, ok := msg["message"].(map[string]interface{}); ok {
			// Check content type
			contentType, hasType := messageContent["type"].(string)
			if !hasType {
				ch.conn.logger.Error("Channel %d: channelSend missing message type", ch.channelID)
				return
			}

			ch.conn.logger.Trace("Channel %d: processing message of type %s", ch.channelID, contentType)

			switch contentType {
			case "progress":
				// Handle progress updates
				if progress, ok := messageContent["progress"].(float64); ok {
					ch.conn.logger.Trace("Channel %d: progress update: %.1f%%", ch.channelID, progress*100)
					ch.updateProgress(progress)
				} else {
					ch.conn.logger.Error("Channel %d: progress message missing valid progress value", ch.channelID)
				}
			case "resolved":
				// Model resolution event - log but don't take any action
				ch.conn.logger.Trace("Channel %d: model resolved", ch.channelID)
			case "success":
				// Model loaded successfully
				ch.conn.logger.Trace("Channel %d: success message received", ch.channelID)
				if info, ok := messageContent["info"].(map[string]interface{}); ok {
					if identifier, ok := info["identifier"].(string); ok {
						ch.conn.logger.Debug("Channel %d: model loaded with identifier %s", ch.channelID, identifier)
						ch.mu.Lock()
						ch.isFinished = true
						ch.mu.Unlock()
						ch.resultCh <- ModelLoadingResult{
							Identifier: identifier,
							Success:    true,
						}
					} else {
						ch.conn.logger.Error("Channel %d: success message info missing identifier", ch.channelID)
						ch.errorCh <- fmt.Errorf("success message info missing identifier")
					}
				} else {
					ch.conn.logger.Error("Channel %d: success message missing info structure", ch.channelID)
					ch.errorCh <- fmt.Errorf("success message missing info structure")
				}
			default:
				ch.conn.logger.Error("Channel %d: unhandled message type: %s", ch.channelID, contentType)
			}
		} else {
			ch.conn.logger.Error("Channel %d: channelSend missing message content", ch.channelID)
		}
	case "channelResolved":
		// Channel has been successfully created
		ch.conn.logger.Debug("Channel %d resolved (created successfully)", ch.channelID)
	case "channelSuccess":
		// Only used in some contexts, not typically for model loading
		ch.conn.logger.Debug("Channel %d success message received", ch.channelID)
		if content, ok := msg["content"].(map[string]interface{}); ok {
			if identifier, ok := content["identifier"].(string); ok {
				ch.mu.Lock()
				ch.isFinished = true
				ch.mu.Unlock()
				ch.resultCh <- ModelLoadingResult{
					Identifier: identifier,
					Success:    true,
				}
			} else {
				ch.conn.logger.Error("Channel %d: channelSuccess missing identifier", ch.channelID)
			}
		} else {
			ch.conn.logger.Error("Channel %d: channelSuccess missing content", ch.channelID)
		}
	case "channelError":
		// Handle error
		ch.conn.logger.Error("Channel %d received error", ch.channelID)
		var errorMsg string
		if content, ok := msg["content"].(map[string]interface{}); ok {
			if err, ok := content["error"].(map[string]interface{}); ok {
				if title, ok := err["title"].(string); ok {
					errorMsg = title
				} else if rootTitle, ok := err["rootTitle"].(string); ok {
					errorMsg = rootTitle
				} else {
					errorMsg = "Unknown channel error"
				}
			} else {
				errorMsg = "Channel error with missing error details"
			}
		} else {
			errorMsg = "Channel error with missing content"
		}
		ch.conn.logger.Error("Channel %d error: %s", ch.channelID, errorMsg)
		ch.errorCh <- fmt.Errorf("model loading error: %s", errorMsg)
	case "channelClose":
		// Channel has been closed
		ch.conn.logger.Trace("Channel %d closed", ch.channelID)
		ch.mu.Lock()
		ch.isFinished = true
		ch.mu.Unlock()
		return
	default:
		ch.conn.logger.Error("Channel %d: unhandled message type: %s", ch.channelID, msgType)
	}
}

// updateProgress updates the progress and calls the progress function if provided
func (ch *ModelLoadingChannel) updateProgress(progress float64) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Ignore if progress goes backwards or repeats
	if progress <= ch.lastProgress {
		return
	}

	ch.lastProgress = progress

	// Call progress function if provided
	if ch.progressFn != nil {
		ch.progressFn(progress)
	}
}

// WaitForResult waits for the model loading to complete
func (ch *ModelLoadingChannel) WaitForResult(timeout time.Duration) (*ModelLoadingResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Log occasional updates during waiting
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case result := <-ch.resultCh:
			ch.conn.logger.Trace("Received success result for channel %d (model: %s)", ch.channelID, ch.modelKey)
			return &result, nil

		case err := <-ch.errorCh:
			ch.conn.logger.Error("Received error for channel %d (model: %s): %v", ch.channelID, ch.modelKey, err)
			return nil, err

		case <-ticker.C:
			// Periodically check for messages from server
			ch.conn.logger.Trace("Channel %d (model: %s) waiting for messages... (%.1f%% done)",
				ch.channelID, ch.modelKey, ch.lastProgress*100)

			// Check connection status
			ch.conn.mu.Lock()
			isConnected := ch.conn.connected
			ch.conn.mu.Unlock()

			if !isConnected {
				return nil, fmt.Errorf("connection lost while waiting for model to load")
			}

		case <-ctx.Done():
			return nil, fmt.Errorf("model loading timed out after %v", timeout)
		}
	}
}

// Close closes the model loading channel
func (ch *ModelLoadingChannel) Close() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.isFinished {
		return nil // Already finished
	}

	// Send channel close message
	closeMsg := map[string]interface{}{
		"type":      "channelClose",
		"channelId": ch.channelID,
	}

	err := ch.conn.conn.WriteJSON(closeMsg)
	if err != nil {
		return fmt.Errorf("failed to close model loading channel: %w", err)
	}

	// Signal the message handler to stop
	close(ch.cancelCh)
	ch.isFinished = true

	return nil
}
