package lmstudio

var (
	LMStudioAPIHosts = []string{"localhost", "127.0.0.1", "0.0.0.0"}
	LMStudioAPIPorts = []int{1234, 12345}
)

const (
	LMStudioWsAPITimeoutSec     = 30
	SystemAPINamespace          = "system"
	LLMNamespace                = "llm"                  // Add LLM namespace for loaded models
	EmbeddingNamespace          = "embedding"            // Add Embedding namespace for embedding models
	ModelListLoadedEndpoint     = "listLoaded"           // Endpoint for listing loaded models
	ModelLoadEndpoint           = "loadModel"            // Endpoint for loading a model
	ModelUnloadEndpoint         = "unloadModel"          // Endpoint for unloading a model
	ModelListDownloadedEndpoint = "listDownloadedModels" // Endpoint for listing downloaded models
	ModelChatEndpoint           = "predict"              // Endpoint for chat/prediction interactions
	MaxConnectionRetries        = 3
	ConnectionRetryDelaySec     = 2
	LMStudioAPIVersion          = 1
)
