package lmstudio

// Model represents a unified model structure for both downloaded and loaded models
type Model struct {
	// Common fields
	ModelKey          string `json:"modelKey"`
	Path              string `json:"path"`
	Type              string `json:"type"`
	Format            string `json:"format,omitempty"`
	Size              int64  `json:"sizeBytes,omitempty"`
	MaxContextLength  int    `json:"maxContextLength,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	Architecture      string `json:"architecture,omitempty"`
	Vision            bool   `json:"vision,omitempty"`
	TrainedForToolUse bool   `json:"trainedForToolUse,omitempty"`

	// Fields specific to loaded models
	Identifier        string `json:"identifier,omitempty"`
	InstanceReference string `json:"instanceReference,omitempty"`
	ContextLength     int    `json:"contextLength,omitempty"`

	// Legacy fields kept for compatibility
	ModelType string `json:"modelType,omitempty"`
	Family    string `json:"family,omitempty"`
	ModelName string `json:"modelName,omitempty"`

	// Internal tracking - not from JSON
	IsLoaded bool `json:"-"`
}

// Add new struct for chat messages
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
