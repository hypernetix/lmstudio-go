package lmstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ChannelHandler is an interface for different types of channel handlers
type ChannelHandler interface {
	handleMessages(wg *sync.WaitGroup)
	processMessage(message []byte)
}

// namespaceConnection represents a connection to a specific LM Studio namespace
type namespaceConnection struct {
	logger             Logger
	namespace          string
	conn               *websocket.Conn
	nextID             int
	pendingCalls       map[int]chan json.RawMessage
	pendingUnloadCalls map[int]bool           // Track unload calls to avoid logging errors for their responses
	activeChannels     map[int]ChannelHandler // Interface for different channel types
	connected          bool
	mu                 sync.Mutex
}

// connect establishes a connection to a specific LM Studio namespace
func (nc *namespaceConnection) connect(apiHost string, parentCtx context.Context) error {

	var u url.URL
	// Build WebSocket URL - match Python SDK's URL structure
	if strings.HasPrefix(apiHost, "https://") {
		apiHost = strings.TrimPrefix(apiHost, "http://")
		u = url.URL{Scheme: "wss", Host: apiHost, Path: "/" + nc.namespace}
	} else if strings.HasPrefix(apiHost, "http://") {
		apiHost = strings.TrimPrefix(apiHost, "http://")
		u = url.URL{Scheme: "ws", Host: apiHost, Path: "/" + nc.namespace}
	} else {
		u = url.URL{Scheme: "ws", Host: apiHost, Path: "/" + nc.namespace}
	}

	// Try to connect with retries
	var conn *websocket.Conn
	var err error

	for retry := 0; retry < MaxConnectionRetries; retry++ {
		if retry > 0 {
			nc.logger.Info("Connection attempt %d/%d after waiting %d seconds...",
				retry+1, MaxConnectionRetries, ConnectionRetryDelaySec)
			time.Sleep(ConnectionRetryDelaySec * time.Second)
		}

		nc.logger.Debug("Connecting to %s", u.String())

		// Configure WebSocket dialer with timeout
		dialer := websocket.DefaultDialer
		dialer.HandshakeTimeout = 15 * time.Second

		// Attempt connection
		conn, _, err = dialer.Dial(u.String(), nil)
		if err == nil {
			break // Connection successful
		}

		nc.logger.Error("Connection attempt failed: %v", err)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to LM Studio after %d attempts: %w",
			MaxConnectionRetries, err)
	}

	nc.conn = conn
	nc.activeChannels = make(map[int]ChannelHandler) // Initialize active channels map

	// Set read deadline for authentication response
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Generate UUIDs for authentication
	clientIdentifier := uuid.New().String()
	clientPasskey := uuid.New().String()

	// Send authentication message exactly matching the Python SDK format
	authMsg := map[string]interface{}{
		"authVersion":      1,
		"clientIdentifier": clientIdentifier,
		"clientPasskey":    clientPasskey,
	}

	nc.logger.Debug("Sending authentication message to %s: %+v", nc.namespace, authMsg)
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send authentication message: %w", err)
	}

	// Read authentication response
	var authResponse map[string]interface{}
	if err := conn.ReadJSON(&authResponse); err != nil {
		conn.Close()
		return fmt.Errorf("authentication failed: %w", err)
	}

	nc.logger.Debug("Received authentication response from %s: %+v", nc.namespace, authResponse)

	// Reset read deadline after authentication
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		conn.Close()
		return fmt.Errorf("failed to reset read deadline: %w", err)
	}

	// Check for success in auth response
	success, ok := authResponse["success"].(bool)
	if !ok || !success {
		conn.Close()
		errorMsg := "unknown error"
		if errDetails, ok := authResponse["error"]; ok {
			errorMsg = fmt.Sprintf("%v", errDetails)
		}
		return fmt.Errorf("authentication failed: %s", errorMsg)
	}

	// Mark as connected and start message handler
	nc.mu.Lock()
	nc.connected = true
	nc.mu.Unlock()

	go nc.handleMessages(parentCtx)

	nc.logger.Debug("Successfully connected and authenticated to %s namespace", nc.namespace)
	return nil
}

// isConnected returns whether the namespace connection is connected
func (nc *namespaceConnection) isConnected() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.connected
}

// close closes the namespace connection
func (nc *namespaceConnection) close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// If not connected or conn is nil, nothing else to do
	if !nc.connected || nc.conn == nil {
		return nil
	}

	// Mark as disconnected
	nc.connected = false

	// Set a deadline to ensure the close completes
	// This helps avoid waiting indefinitely if the server doesn't respond
	_ = nc.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

	// Send a close message
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	// Ignore error if connection is already broken
	_ = nc.conn.WriteMessage(websocket.CloseMessage, closeMsg)

	// Give time for the close message to be sent and processed
	time.Sleep(250 * time.Millisecond)

	// Now close the connection
	return nc.conn.Close()
}

// handleMessages handles incoming WebSocket messages for a namespace
func (nc *namespaceConnection) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Context was cancelled, exit silently
			return
		default:
			_, message, err := nc.conn.ReadMessage()
			if err != nil {
				// Check if we're shutting down
				select {
				case <-ctx.Done():
					// Context was cancelled, exit silently without logging the error
					nc.mu.Lock()
					nc.connected = false
					nc.mu.Unlock()
					return
				default:
					// Only log unexpected errors during normal operation
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure,
						websocket.CloseGoingAway, websocket.CloseNoStatusReceived) &&
						!strings.Contains(err.Error(), "use of closed network connection") &&
						!strings.Contains(err.Error(), "websocket: close sent") {
						nc.logger.Error("Error reading message from %s: %v", nc.namespace, err)
					}
				}

				nc.mu.Lock()
				nc.connected = false
				nc.mu.Unlock()
				return
			}

			// For debugging, print the raw message
			nc.logger.Debug("Received raw WebSocket message from %s: %s", nc.namespace, string(message))

			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err != nil {
				nc.logger.Error("Error parsing message from %s: %v", nc.namespace, err)
				continue
			}

			// Check message type
			msgType, hasType := msg["type"].(string)
			if !hasType {
				nc.logger.Error("Message has no type field from %s", nc.namespace)
				continue
			}

			nc.logger.Trace("Processing message type '%s' from %s", msgType, nc.namespace)

			// Handle special communication warnings globally
			if msgType == "communicationWarning" {
				if warning, ok := msg["warning"].(string); ok {
					nc.logger.Warn("WARNING: Communication issue from %s: %s", nc.namespace, warning)
				}
				continue
			}

			// Handle RPC messages
			if msgType == "rpcResult" || msgType == "rpcError" {
				if callID, ok := msg["callId"].(float64); ok {
					nc.mu.Lock()
					callIDInt := int(callID)

					// Check if this is a response to an unload call that we should ignore
					isUnloadCall := nc.pendingUnloadCalls != nil && nc.pendingUnloadCalls[callIDInt]
					if isUnloadCall {
						// Clean up the pending unload call tracking
						delete(nc.pendingUnloadCalls, callIDInt)
						nc.logger.Debug("Received response for unload call ID %d from %s (ignoring)", callIDInt, nc.namespace)
						nc.mu.Unlock()
						continue
					}

					if ch, exists := nc.pendingCalls[callIDInt]; exists {
						if msgType == "rpcResult" {
							ch <- message
							delete(nc.pendingCalls, callIDInt)
						} else if msgType == "rpcError" {
							// Handle error responses
							if errObj, ok := msg["error"].(map[string]interface{}); ok {
								title := "Unknown error"
								if t, ok := errObj["title"].(string); ok {
									title = t
								} else if rt, ok := errObj["rootTitle"].(string); ok {
									title = rt
								}
								// Log the error title
								nc.logger.Error("RPC error from %s: %s", nc.namespace, title)

								// Also log the detailed error with pretty formatting
								var prettyJSON bytes.Buffer
								if err := json.Indent(&prettyJSON, message, "", "  "); err == nil {
									nc.logger.Trace("RPC error details from %s: \n%s", nc.namespace, prettyJSON.String())
								}
							}
							ch <- message
							delete(nc.pendingCalls, callIDInt)
						}
					} else {
						nc.logger.Error("Received response for unknown call ID %d from %s", callIDInt, nc.namespace)
					}
					nc.mu.Unlock()
					continue
				}
			}

			// Check if this is a channel message and route it to the appropriate channel handler
			if strings.HasPrefix(msgType, "channel") {
				channelID, hasChannel := msg["channelId"].(float64)
				if !hasChannel {
					nc.logger.Error("Channel message missing channelId from %s: %s", nc.namespace, msgType)
					continue
				}

				// Find the channel handler
				nc.mu.Lock()
				channelHandler, exists := nc.activeChannels[int(channelID)]
				nc.mu.Unlock()

				if exists {
					nc.logger.Trace("Routing %s message to channel %d", msgType, int(channelID))

					// Forward message to appropriate channel type
					if loadingChannel, ok := channelHandler.(*ModelLoadingChannel); ok {
						select {
						case loadingChannel.messageCh <- message:
							// Message sent successfully
							nc.logger.Trace("Message sent to ModelLoadingChannel %d", int(channelID))
						default:
							// Channel buffer full, this shouldn't happen with normal usage
							nc.logger.Error("Message buffer full for loading channel %d, dropping message", int(channelID))
						}
					} else if streamingChannel, ok := channelHandler.(*ModelStreamingChannel); ok {
						select {
						case streamingChannel.messageCh <- message:
							// Message sent successfully
							nc.logger.Trace("Message sent to ModelStreamingChannel %d", int(channelID))
						default:
							// Channel buffer full, this shouldn't happen with normal usage
							nc.logger.Error("Message buffer full for streaming channel %d, dropping message", int(channelID))
						}
					} else {
						nc.logger.Error("Unknown channel handler type for channel %d", int(channelID))
					}
				} else {
					nc.logger.Error("Received message for unknown channel %d from %s", int(channelID), nc.namespace)
				}
				continue
			}

			// For other messages, log with pretty formatting
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, message, "", "  "); err == nil {
				nc.logger.Trace("Received other message from %s: \n%s", nc.namespace, prettyJSON.String())
			} else {
				// Fallback to original format if pretty printing fails
				nc.logger.Trace("Received other message from %s: %s", nc.namespace, string(message))
			}
		}
	}
}

// ensureConnected ensures the namespace connection is connected or returns an error
func (nc *namespaceConnection) ensureConnected() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if !nc.connected || nc.conn == nil {
		return fmt.Errorf("not connected to %s namespace", nc.namespace)
	}
	return nil
}

// RemoteCall makes a remote procedure call to a specific namespace
func (nc *namespaceConnection) RemoteCall(endpoint string, params interface{}) (json.RawMessage, error) {
	if err := nc.ensureConnected(); err != nil {
		return nil, err
	}

	nc.mu.Lock()
	id := nc.nextID
	nc.nextID++
	ch := make(chan json.RawMessage, 1)
	nc.pendingCalls[id] = ch
	nc.mu.Unlock()

	// Create RPC message in the format exactly matching the Python SDK
	rpcMsg := map[string]interface{}{
		"type":     "rpcCall",
		"endpoint": endpoint,
		"callId":   id,
	}

	// Add params if provided - also directly in the message
	if params != nil {
		rpcMsg["parameter"] = params // Use "parameter" singular as in Python SDK
	}

	// Log the exact RPC message for debugging
	rpcMsgBytes, _ := json.Marshal(rpcMsg)
	nc.logger.Debug("Sending RPC call to %s: %s", nc.namespace, string(rpcMsgBytes))

	if err := nc.conn.WriteJSON(rpcMsg); err != nil {
		nc.mu.Lock()
		delete(nc.pendingCalls, id)
		nc.mu.Unlock()
		return nil, fmt.Errorf("failed to send RPC message: %w", err)
	}

	// Wait for response with timeout
	ctx, cancel := context.WithTimeout(context.Background(), LMStudioWsAPITimeoutSec*time.Second)
	defer cancel()

	select {
	case response := <-ch:
		// Pretty-print the response for better readability
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, response, "", "  "); err == nil {
			nc.logger.Debug("Received RPC response from %s: \n%s", nc.namespace, prettyJSON.String())
		} else {
			// Fallback to original format if pretty printing fails
			nc.logger.Debug("Received RPC response from %s: %s", nc.namespace, string(response))
		}

		var respMap map[string]interface{}
		if err := json.Unmarshal(response, &respMap); err != nil {
			return nil, fmt.Errorf("failed to parse RPC response: %w", err)
		}

		// Check response type
		responseType, ok := respMap["type"].(string)
		if !ok {
			return nil, fmt.Errorf("missing response type")
		}

		// Handle error responses
		if responseType == "rpcError" {
			errObj, ok := respMap["error"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("malformed error response")
			}

			// Extract error message from various possible fields
			var errorMsg string
			if title, ok := errObj["title"].(string); ok && title != "" {
				errorMsg = title
			} else if rootTitle, ok := errObj["rootTitle"].(string); ok && rootTitle != "" {
				errorMsg = rootTitle
			} else if msg, ok := errObj["message"].(string); ok && msg != "" {
				errorMsg = msg
			} else {
				errorMsg = "Unknown RPC error"
			}

			return nil, fmt.Errorf("%s", errorMsg)
		}

		// Handle success responses
		if responseType == "rpcResult" {
			if result, ok := respMap["result"]; ok {
				resultBytes, err := json.Marshal(result)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal result: %w", err)
				}
				return resultBytes, nil
			}
			// Empty result is valid for some operations
			return json.RawMessage("null"), nil
		}

		return nil, fmt.Errorf("unexpected response type: %s", responseType)

	case <-ctx.Done():
		nc.mu.Lock()
		delete(nc.pendingCalls, id)
		nc.mu.Unlock()
		return nil, errors.New("RPC call timed out")
	}
}
