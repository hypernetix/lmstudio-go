# LM Studio Go

A Go library and CLI for interacting with [LM Studio](https://lmstudio.ai/), inspired by [lmstudio-python](https://github.com/lmstudio-ai/lmstudio-client-python).


## Motivation

LM Studio is a great tool for interacting with LLMs and it has REST API for chatting, models management and more. However, this API is incomplete and is still in Beta, for example it doesn't yet support:

- Advanced model details (size, path, etc)
- Model loading and unloading progress
- Server management (start, stop, etc)


## Overview

This library provides Go bindings for LM Studio, allowing you to interact with LM Studio's WebSocket API from Go applications. It supports:

- Status check to verify if the LM Studio service is running
- Version reporting (version 1.0)
- Model management:
  - Listing loaded LLM models
  - Listing loaded embedding models
  - Listing all loaded models (LLMs and embeddings)
  - Listing downloaded models
  - Loading specific models
  - Unloading specific models
  - Unloading all loaded models
- Sending prompts to models with streaming responses
- Configurable logging with multiple log levels


## Installation

```bash
go get github.com/hypernetix/lmstudio-go
```


## Library Usage

### Basic Example

```go
package main

import (
	"fmt"

	"github.com/hypernetix/lmstudio-go/pkg/lmstudio"
)

func main() {
	// Create a logger with desired verbosity
	logger := lmstudio.NewLogger(lmstudio.LogLevelInfo)

	// Create an LM Studio client
	client := lmstudio.NewLMStudioClient("localhost:1234", logger)
	defer client.Close()

	// List all loaded models
	models, err := client.ListLoadedLLMs()
	if err != nil {
		logger.Error("Failed to list loaded models: %v", err)
		return
	}

	// Print the models
	fmt.Println("Loaded models:")
	for _, model := range models {
		fmt.Printf("- %s\n", model.Identifier)
	}

	// Load a model if none is loaded
	if len(models) == 0 {
		downloaded, err := client.ListDownloadedModels()
		if err != nil || len(downloaded) == 0 {
			logger.Error("No models available")
			return
		}

		modelToLoad := downloaded[0].ModelKey
		fmt.Printf("Loading model: %s\n", modelToLoad)
		if err := client.LoadModel(modelToLoad); err != nil {
			logger.Error("Failed to load model: %v", err)
			return
		}

		// Update the models list
		models, _ = client.ListLoadedLLMs()
	}

	// Send a prompt to the first loaded model
	if len(models) > 0 {
		modelID := models[0].Identifier
		prompt := "Tell me a short joke"

		fmt.Printf("\nSending prompt to %s: %s\n\nResponse:\n", modelID, prompt)

		// Create a callback to print tokens as they arrive
		callback := func(token string) {
			fmt.Print(token)
		}

		if err := client.SendPrompt(modelID, prompt, 0.7, callback); err != nil {
			logger.Error("Failed to send prompt: %v", err)
		}
		fmt.Println("\n")
	}
}
```

### Custom Logger

You can implement your own logger by implementing the `lmstudio.Logger` interface:

```go
type MyLogger struct {
	level lmstudio.LogLevel
}

func (l *MyLogger) SetLevel(level lmstudio.LogLevel) {
	l.level = level
}

func (l *MyLogger) Error(format string, v ...interface{}) {
	// Your custom error logging implementation
}

// Implement other required methods: Warn, Info, Debug, Trace

// Then use it with the client
client := lmstudio.NewLMStudioClient("localhost:1234", &MyLogger{level: lmstudio.LogLevelDebug})
```


## CLI Usage

The project includes a command-line interface for interacting with LM Studio:

```bash
# Check if LM Studio service is running
lms-go --status

# Display version information
lms-go --version

# List all loaded models
lms-go --list

# List loaded LLM models
lms-go --list-llms

# List loaded embedding models
lms-go --list-embeddings

# List downloaded models
lms-go --list-downloaded

# Load a model
lms-go --load="mistral-7b-instruct"

# Unload a model
lms-go --unload="mistral-7b-instruct"

# Unload all loaded models
lms-go --unload-all

# Send a prompt to a model
lms-go --model="mistral-7b-instruct" --prompt="Tell me a joke" --temp=0.7

# Enable verbose logging
lms-go -v

# Wait for Ctrl+C to exit (useful for keeping the program running)
lms-go --wait
```


## API Reference

### Client Initialization

```go
// Create a new client with default logger
client := lmstudio.NewLMStudioClient("localhost:1234", nil)

// Create a new client with custom logger and log level
logger := lmstudio.NewLogger(lmstudio.LogLevelDebug)
client := lmstudio.NewLMStudioClient("localhost:1234", logger)
```

### Model Management

```go
// List loaded LLM models
models, err := client.ListLoadedLLMs()

// List loaded embedding models
models, err := client.ListLoadedEmbeddingModels()

// List all loaded models (LLMs and embeddings)
models, err := client.ListAllLoadedModels()

// List downloaded models
models, err := client.ListDownloadedModels()

// Load a model
err := client.LoadModel("mistral-7b-instruct")

// Unload a model
err := client.UnloadModel("mistral-7b-instruct")

// Unload all loaded models
err := client.UnloadAllModels()
```

### Inference

```go
// Send a prompt with streaming response
callback := func(token string) {
    fmt.Print(token)
}
err := client.SendPrompt("mistral-7b-instruct", "Tell me a joke", 0.7, callback)
```

### Service Status

```go
// Check if LM Studio service is running
running, err := client.IsServiceRunning()
if running {
    fmt.Println("LM Studio service is running")
} else {
    fmt.Println("LM Studio service is NOT running")
}
```

### Cleanup

```go
// Close the client and all connections
client.Close()
```


## Project Structure

- `main.go`: example of CLI executable entry point
- `pkg/lmstudio/`: Library package code
  - `lmstudio_client.go`: Main client implementation
  - `conn.go`: Namespace connection and WebSocket management
  - `model_loading.go`: Model loading channel and related logic
  - `chat_streaming.go`: Streaming chat/channel logic
  - `logger.go`: Logging abstraction with multiple log levels
  - `types.go`: Data types (e.g., `Model`, `ChatMessage`, etc.)
  - `params.go`: Parameter structures for API calls


## Testing

```bash
make test
```

## Using the Makefile

The project includes a Makefile to simplify common development tasks:

```bash
# Run tests, generate coverage report, and build the binary
make all

# Run all the tests
make tests

# Run only the unit tests
make unit-tests

# Run only the CLI tests
make cli-tests

# Generate code coverage reports
make coverage

# Build the CLI executable (outputs to build/lms-go)
make build

# Install the CLI executable to your Go bin directory
make install

# Clean up build artifacts and coverage files
make clean
```


## License

[Apache License 2.0](LICENSE.md)
