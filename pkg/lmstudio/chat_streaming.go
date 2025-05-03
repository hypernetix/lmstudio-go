package lmstudio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

// ModelStreamingChannel handles streaming messages from models
type ModelStreamingChannel struct {
	channelID int
	conn      *namespaceConnection
	streamCh  chan string
	errCh     chan error
	doneCh    chan struct{}
	messageCh chan []byte
}

// NewModelStreamingChannel creates a new channel for streaming responses
func NewModelStreamingChannel(channelID int, conn *namespaceConnection, streamCh chan string, errCh chan error, doneCh chan struct{}) *ModelStreamingChannel {
	return &ModelStreamingChannel{
		channelID: channelID,
		conn:      conn,
		streamCh:  streamCh,
		errCh:     errCh,
		doneCh:    doneCh,
		messageCh: make(chan []byte, 50), // Buffer for incoming messages
	}
}

// processMessage processes a message received from the namespace handler
func (ch *ModelStreamingChannel) processMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		ch.conn.logger.Error("Error parsing message on channel %d: %v", ch.channelID, err)
		return
	}

	// Debug the message format for troubleshooting
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, message, "", "  "); err == nil {
		ch.conn.logger.Trace("Streaming channel %d received message: \n%s", ch.channelID, prettyJSON.String())
	} else {
		ch.conn.logger.Trace("Streaming channel %d received message: %s", ch.channelID, string(message))
	}

	// Get message type
	msgType, ok := msg["type"].(string)
	if !ok {
		ch.conn.logger.Error("Channel %d: Message missing type field", ch.channelID)
		return
	}

	ch.conn.logger.Trace("Processing streaming message of type: %s", msgType)

	// Handle streaming messages
	if msgType == "channelSend" {
		if messageContent, ok := msg["message"].(map[string]interface{}); ok {
			contentType, hasType := messageContent["type"].(string)
			if !hasType {
				ch.conn.logger.Error("Channel %d: channelSend missing message type", ch.channelID)
				return
			}

			ch.conn.logger.Trace("Channel %d processing message of type: %s", ch.channelID, contentType)

			switch contentType {
			case "chatToken", "fragment":
				// Extract the token from prediction fragment
				var token string
				if contentType == "chatToken" {
					// Format from our original implementation
					if t, ok := messageContent["token"].(string); ok {
						token = t
						ch.conn.logger.Trace("Extracted chatToken: %s", token)
					} else {
						ch.conn.logger.Error("Failed to extract token from chatToken message")
					}
				} else if contentType == "fragment" {
					// Format from Python SDK's predict endpoint
					// Check for the nested fragment object
					if fragmentObj, ok := messageContent["fragment"].(map[string]interface{}); ok {
						if content, ok := fragmentObj["content"].(string); ok {
							token = content
							ch.conn.logger.Trace("Extracted fragment content: %s", token)
						} else {
							ch.conn.logger.Error("Fragment object has no content field or it's not a string")
						}
					} else {
						ch.conn.logger.Error("Fragment message has no fragment object")
					}
				}

				if token != "" {
					// Send token to stream channel
					ch.conn.logger.Trace("Sending token to stream channel: %s", token)
					select {
					case ch.streamCh <- token:
						ch.conn.logger.Trace("Token sent to stream channel successfully")
					default:
						ch.conn.logger.Error("Failed to send token to stream channel (buffer full)")
					}
				} else {
					ch.conn.logger.Trace("Empty token, not sending to stream: %v", messageContent)
				}
			case "chatEnd", "completed":
				// Chat has ended
				ch.conn.logger.Debug("Chat completed")
				close(ch.doneCh)
			}
		} else {
			ch.conn.logger.Error("Channel %d: channelSend missing message content", ch.channelID)
		}
	} else if msgType == "channelError" {
		// Handle error
		var errorMsg string
		if content, ok := msg["content"].(map[string]interface{}); ok {
			if err, ok := content["error"].(map[string]interface{}); ok {
				if title, ok := err["title"].(string); ok {
					errorMsg = title
				} else {
					errorMsg = "Unknown channel error"
				}
			}
		}
		ch.conn.logger.Error("Channel %d error: %s", ch.channelID, errorMsg)
		ch.errCh <- fmt.Errorf("chat error: %s", errorMsg)
	} else if msgType == "channelSuccess" || msgType == "channelClose" {
		// Channel was closed or completed successfully
		ch.conn.logger.Trace("Channel %d closed or completed: %s", ch.channelID, msgType)
		close(ch.doneCh)
	}
}

// handleMessages processes messages for the streaming channel
func (ch *ModelStreamingChannel) handleMessages(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	ch.conn.logger.Debug("Started message handler for streaming channel %d", ch.channelID)
	messageCount := 0

	for {
		select {
		case <-ch.doneCh:
			// Channel is closed
			ch.conn.logger.Trace("Streaming channel %d closed, processed %d messages", ch.channelID, messageCount)
			return
		case message := <-ch.messageCh:
			// Process incoming message
			messageCount++
			ch.conn.logger.Trace("Streaming channel %d received message #%d", ch.channelID, messageCount)
			ch.processMessage(message)
		}
	}
}
