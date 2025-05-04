package lmstudio

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

type mockModel struct {
	ModelKey          string
	Identifier        string
	InstanceReference string
	Path              string
	Type              string
	IsLoaded          bool
}

var mockModel1 = mockModel{
	ModelKey:          "mock-model-0.5B",
	Identifier:        "mock-model-0.5B",
	InstanceReference: "mock-instance-ref",
	Path:              "/mock/path/to/model",
	Type:              "llm",
	IsLoaded:          false,
}

var mockModel2 = mockModel{
	ModelKey:          "mock-model-7B",
	Identifier:        "mock-model-7B",
	InstanceReference: "mock-instance-ref",
	Path:              "/mock/path/to/model",
	Type:              "llm",
	IsLoaded:          true,
}

// NewMockLMStudioService creates a test WebSocket server for unit testing.
// Handles LM Studio API endpoints as used in the test suite.
func NewMockLMStudioService(t *testing.T, logger Logger) *httptest.Server {
	t.Helper()

	var mockModels = []mockModel{
		mockModel1,
		mockModel2,
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		// Handle authentication first
		var authMsg map[string]interface{}
		if err := conn.ReadJSON(&authMsg); err != nil {
			t.Fatalf("Failed to read auth message: %v", err)
			return
		}

		// Send auth response
		authResponse := map[string]interface{}{
			"success": true,
		}
		if err := conn.WriteJSON(authResponse); err != nil {
			t.Fatalf("Failed to write auth response: %v", err)
			return
		}

		// Main handler loop: process API requests
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				logger.Warn("Mock server: connection closed or error: %v", err)
				return
			}

			endpoint, _ := msg["endpoint"].(string)
			msgType, _ := msg["type"].(string)

			logger.Debug("Mock server: received message: %v\n", msg)
			switch {
			case msgType == "rpcCall" && endpoint == "listLoaded":
				// Respond with a single loaded model
				loadedModels := []mockModel{}
				for _, model := range mockModels {
					if model.IsLoaded {
						loadedModels = append(loadedModels, model)
					}
				}
				response := map[string]interface{}{
					"type":   "rpcResult",
					"callId": msg["callId"],
					"result": loadedModels,
				}
				if err := conn.WriteJSON(response); err != nil {
					t.Fatalf("Failed to write listLoaded response: %v", err)
				}
			case msgType == "rpcCall" && endpoint == "listDownloadedModels":
				// Respond with two downloaded models
				response := map[string]interface{}{
					"type":   "rpcResult",
					"callId": msg["callId"],
					"result": mockModels,
				}
				if err := conn.WriteJSON(response); err != nil {
					t.Fatalf("Failed to write listDownloadedModels response: %v", err)
				}
			case msgType == "rpcCall" && endpoint == "unloadModel":
				// Respond with success
				params := msg["parameter"].(map[string]interface{})
				modelKey := params["identifier"].(string)
				for i, model := range mockModels {
					if model.ModelKey == modelKey {
						mockModels[i].IsLoaded = false
						break
					}
				}
				response := map[string]interface{}{
					"type":   "rpcResult",
					"callId": msg["callId"],
					"result": map[string]interface{}{"success": true},
				}
				if err := conn.WriteJSON(response); err != nil {
					t.Fatalf("Failed to write unloadModel response: %v", err)
				}
			case msgType == "channelCreate" && endpoint == "loadModel":
				params := msg["creationParameter"].(map[string]interface{})
				modelKey := params["identifier"].(string)
				var model *mockModel
				for i := range mockModels {
					if mockModels[i].ModelKey == modelKey {
						mockModels[i].IsLoaded = true
						model = &mockModels[i]
						break
					}
				}

				if model == nil {
					response := map[string]interface{}{
						"type":      "channelError",
						"channelId": msg["channelId"],
						"error": map[string]interface{}{
							"code":    404,
							"message": "Model not found",
						},
					}
					if err := conn.WriteJSON(response); err != nil {
						t.Fatalf("Failed to write channel error: %v", err)
					}
					return
				}

				// Simulate progress and success for model loading
				channelID, _ := msg["channelId"].(float64)
				progressSteps := []float64{0.1, 0.3, 0.5, 0.7, 0.9, 1.0}
				for _, progress := range progressSteps {
					progressMsg := map[string]interface{}{
						"type":      "channelSend",
						"channelId": int(channelID),
						"message": map[string]interface{}{
							"type":     "progress",
							"progress": progress,
						},
					}
					if err := conn.WriteJSON(progressMsg); err != nil {
						t.Fatalf("Failed to write progress message: %v", err)
					}
				}
				// Send a 'success' message as expected by the client logic
				successMsg := map[string]interface{}{
					"type":      "channelSend",
					"channelId": int(channelID),
					"message": map[string]interface{}{
						"type": "success",
						"info": map[string]interface{}{
							"identifier":        model.Identifier,
							"instanceReference": model.InstanceReference,
						},
					},
				}
				if err := conn.WriteJSON(successMsg); err != nil {
					t.Fatalf("Failed to write success message: %v", err)
				}
			case msgType == "channelCreate" && endpoint == "predict":
				params := msg["creationParameter"].(map[string]interface{})
				modelSpecifier := params["modelSpecifier"].(map[string]interface{})
				instanceReference := modelSpecifier["instanceReference"].(string)
				var model *mockModel
				for i := range mockModels {
					if mockModels[i].InstanceReference == instanceReference {
						model = &mockModels[i]
						break
					}
				}

				if model == nil {
					response := map[string]interface{}{
						"type":      "channelError",
						"channelId": msg["channelId"],
						"error": map[string]interface{}{
							"code":    404,
							"message": "Model not found",
						},
					}
					if err := conn.WriteJSON(response); err != nil {
						t.Fatalf("Failed to write channel error: %v", err)
					}
					return
				}

				// Simulate streaming tokens for chat
				channelID, _ := msg["channelId"].(float64)
				tokens := []string{"Hello", ", ", "world", "!"}
				for _, token := range tokens {
					channelMsg := map[string]interface{}{
						"type":      "channelSend",
						"channelId": int(channelID),
						"message": map[string]interface{}{
							"type": "fragment",
							"fragment": map[string]interface{}{
								"content": token,
							},
						},
					}
					logger.Debug("Mock server: Sending token '%s' to channel %d", token, int(channelID))
					if err := conn.WriteJSON(channelMsg); err != nil {
						t.Fatalf("Failed to write channel message: %v", err)
					}
				}
				doneMsg := map[string]interface{}{
					"type":      "channelSend",
					"channelId": int(channelID),
					"message": map[string]interface{}{
						"type": "success",
					},
				}

				logger.Debug("Mock server: Sending channel done (success) message for channel %d", int(channelID))
				if err := conn.WriteJSON(doneMsg); err != nil {
					t.Fatalf("Failed to write done message: %v", err)
				}

				logger.Debug("Mock server: Sending channel close message for channel %d", int(channelID))
				closeMsg := map[string]interface{}{
					"type":      "channelClose",
					"channelId": int(channelID),
				}
				if err := conn.WriteJSON(closeMsg); err != nil {
					t.Fatalf("Failed to write channel close message: %v", err)
				}
			default:
				t.Logf("Mock server received unhandled message: %v", msg)
			}
		}
	}))

	return server
}
