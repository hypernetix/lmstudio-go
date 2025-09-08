package main

import (
	"context"
	"encoding/json" // Added this import
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hypernetix/lmstudio-go/pkg/lmstudio"
)

// coverageFile is set at build time via -ldflags for instrumented builds
var coverageFile string

// Global quiet flag
var quietMode bool

// quietPrintf prints to stdout only if not in quiet mode
func quietPrintf(format string, args ...interface{}) {
	if !quietMode {
		fmt.Printf(format, args...)
	}
}

// quietPrintln prints to stdout only if not in quiet mode
func quietPrintln(args ...interface{}) {
	if !quietMode {
		fmt.Println(args...)
	}
}

// formatSize formats file size in a human-readable format
func formatSize(size int64) string {
	if size == 0 {
		return "N/A"
	}

	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// formatMaxContext formats max context length in human-readable format
func formatMaxContext(maxContext int) string {
	if maxContext == 0 {
		return "N/A"
	}
	if maxContext >= 1000 {
		return fmt.Sprintf("%dk", maxContext/1000)
	}
	return fmt.Sprintf("%d", maxContext)
}

// printTableHeader prints a table header with specified column widths
func printTableHeader(columns []string, widths []int) {
	// Print header
	for i, col := range columns {
		quietPrintf("%-*s", widths[i], col)
		if i < len(columns)-1 {
			quietPrintf(" | ")
		}
	}
	quietPrintln()

	// Print separator
	totalWidth := 0
	for i, width := range widths {
		for j := 0; j < width; j++ {
			quietPrintf("-")
		}
		if i < len(widths)-1 {
			quietPrintf("-+-")
			totalWidth += width + 3
		} else {
			totalWidth += width
		}
	}
	quietPrintln()
}

// printModels prints models in a nice table format or JSON
func printModels(models []lmstudio.Model, title string, jsonOutput bool) { // Added jsonOutput parameter
	if jsonOutput {
		jsonData, err := json.MarshalIndent(models, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling to JSON: %v\n", err)
			os.Exit(1) // Or handle error more gracefully
		}
		fmt.Println(string(jsonData)) // Always print JSON output regardless of quiet mode
		return
	}

	quietPrintf("\n%s:\n", title)
	if len(models) == 0 {
		quietPrintf("No %s found\n", strings.ToLower(title))
		return
	}

	// Determine the longest model name for formatting
	longestModelName := 0
	for _, model := range models {
		nameToCheck := model.ModelKey
		if model.IsLoaded && model.Identifier != "" {
			nameToCheck = model.Identifier
		} else if model.DisplayName != "" {
			nameToCheck = model.DisplayName
		} else if model.ModelName != "" {
			nameToCheck = model.ModelName
		}

		if len(nameToCheck) > longestModelName {
			longestModelName = len(nameToCheck)
		}
	}

	// Ensure minimum width and add padding
	longestModelName = max(longestModelName, 15) + 2

	// Define the table format
	columns := []string{"Name", "Type", "Format", "Size", "Context", "Path"}
	widths := []int{longestModelName, 15, 10, 10, 10, 50}

	printTableHeader(columns, widths)

	for _, model := range models {
		// Determine the name to display
		name := ""
		if model.ModelKey != "" {
			name = model.ModelKey
		} else if model.Identifier != "" {
			name = model.Identifier
		} else if model.DisplayName != "" {
			name = model.DisplayName
		} else if model.ModelName != "" {
			name = model.ModelName
		}

		// Determine the type
		modelType := model.Type
		if modelType == "" && model.ModelType != "" {
			modelType = model.ModelType
		}
		if modelType == "" {
			modelType = "N/A"
		}

		// Determine the format
		format := model.Format
		if format == "" {
			// Try to infer format from path
			if path := model.Path; path != "" {
				if strings.Contains(path, "GGUF") || strings.Contains(path, ".gguf") {
					format = "GGUF"
				} else if strings.Contains(path, "GGML") || strings.Contains(path, ".ggml") {
					format = "GGML"
				} else if strings.Contains(path, "MLX") {
					format = "MLX"
				} else if strings.Contains(path, "safetensors") {
					format = "safetensors"
				} else {
					format = "N/A"
				}
			} else {
				format = "N/A"
			}
		}

		// Print the row
		quietPrintf("%-*s | %-15s | %-10s | %-10s | %-10s | %-50s\n",
			longestModelName,
			truncateString(name, longestModelName),
			truncateString(modelType, 15),
			truncateString(format, 10),
			formatSize(model.Size),
			formatMaxContext(model.MaxContextLength),
			truncateString(model.Path, 50))
	}
}

// truncateString truncates a string if it's longer than maxLen and adds "..."
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// loadModelWithProgress loads a model and displays a progress bar with model information
func loadModelWithProgress(client *lmstudio.LMStudioClient, loadTimeout time.Duration, modelIdentifier string, logger lmstudio.Logger) error {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for Ctrl+C cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Clean up signal handler

	// Channel to communicate completion or error
	done := make(chan error, 1)

	// Start the loading process in a goroutine
	go func() {
		var modelInfo *lmstudio.Model
		var modelDisplayed bool
		var lastProgress float64 = -1

		// Use the client's LoadModelWithProgressContext method
		err := client.LoadModelWithProgressContext(ctx, loadTimeout, modelIdentifier, func(progress float64, info *lmstudio.Model) {
			// Display model info on first callback
			if !modelDisplayed {
				modelInfo = info
				if modelInfo != nil {
					format := modelInfo.Format
					if modelInfo.Size > 0 {
						// Extract format from model info for display
						if format == "" && modelInfo.Path != "" {
							if strings.Contains(modelInfo.Path, "MLX") {
								format = "MLX"
							} else if strings.Contains(modelInfo.Path, "GGUF") {
								format = "GGUF"
							}
						}
					}
					quietPrintf("Loading model \"%s\" (size: %s, format: %s) ...\n", modelInfo.ModelKey, formatSize(modelInfo.Size), format)
				} else {
					quietPrintf("Loading model \"%s\" ...\n", modelIdentifier)
				}
				modelDisplayed = true
			}

			// Only update progress if it increased significantly to avoid flickering
			if progress > lastProgress+0.001 || progress >= 1.0 {
				displayProgressBar(progress)
				lastProgress = progress
			}

			// If model was already loaded, show completion immediately
			if progress >= 1.0 {
				quietPrintf("\n[SUCCESS] Model loaded successfully\n")
			}
		})

		done <- err
	}()

	// Wait for either completion or cancellation signal
	select {
	case err := <-done:
		// Loading completed (successfully or with error)
		if err != nil {
			if strings.Contains(err.Error(), "timed out") {
				quietPrintf("\n[TIMEOUT] Model loading timed out\n")
			} else if strings.Contains(err.Error(), "cancelled") {
				quietPrintf("\n[WARNING] Model loading cancelled\n")
			} else {
				quietPrintf("\nFailed to load model: %v\n", err)
			}
			return err
		}
		return nil

	case sig := <-sigChan:
		// User pressed Ctrl+C or sent termination signal
		logger.Debug("Received signal: %v", sig)

		// Clear progress bar if displayed
		if !quietMode {
			fmt.Printf("\r%s\r", strings.Repeat(" ", 80)) // Clear the line
		}

		quietPrintf("\n[WARNING] Model loading cancelled by user\n")

		// Cancel the context to stop the loading operation
		logger.Debug("Cancelling context to stop loading operation...")
		cancel()

		// Wait a longer time for graceful cancellation (5 seconds instead of 2)
		logger.Debug("Waiting up to 5 seconds for graceful cancellation...")
		select {
		case err := <-done:
			// Loading operation acknowledged the cancellation
			logger.Debug("Loading operation gracefully cancelled with result: %v", err)
		case <-func() <-chan struct{} {
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*5)
			defer timeoutCancel()
			return timeoutCtx.Done()
		}():
			// Timeout waiting for graceful cancellation
			logger.Debug("Timeout waiting for loading cancellation after 5 seconds")
		}

		logger.Debug("Cancellation process completed, returning error")
		return fmt.Errorf("model loading cancelled by user")
	}
}

// displayProgressBar shows a progress bar similar to the screenshot
func displayProgressBar(progress float64) {
	if quietMode {
		return // Don't display progress bar in quiet mode
	}

	const barWidth = 50
	percentage := progress * 100

	// Calculate number of filled characters
	filled := int(progress * float64(barWidth))

	// Build the progress bar using block characters like in the screenshot
	bar := make([]rune, barWidth)
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar[i] = '█' // Full block
		} else {
			bar[i] = '░' // Light shade
		}
	}

	// Print progress bar with percentage (carriage return to overwrite)
	fmt.Printf("\r: [%s] %.2f%%", string(bar), percentage)

	// Force output to be displayed immediately
	os.Stdout.Sync()
}

func main() {
	// Setup code coverage if running instrumented build
	if coverageFile != "" {
		quietPrintf("Running with code coverage. Data will be written to: %s\n", coverageFile)
	}

	// Define command-line flags
	host := flag.String("host", "", fmt.Sprintf("LM Studio API host (default: %s)", lmstudio.LMStudioAPIHosts[0]))
	port := flag.Int("port", 0, fmt.Sprintf("LM Studio API port (default: %d)", lmstudio.LMStudioAPIPorts[0]))
	listLoaded := flag.Bool("list-loaded", false, "List all loaded models")
	listLoadedLLMs := flag.Bool("list-loaded-llms", false, "List loaded LLM models")
	listLoadedEmbeddings := flag.Bool("list-loaded-embeddings", false, "List loaded embedding models")
	listDownloaded := flag.Bool("list-downloaded", false, "List all downloaded models")
	loadModel := flag.String("load", "", "Load a model by name or path")
	unloadModel := flag.String("unload", "", "Unload a model by name or identifier")
	unloadAll := flag.Bool("unload-all", false, "Unload all loaded models")
	promptText := flag.String("prompt", "", "Send a prompt to the model")
	promptModel := flag.String("model", "", "Model to use for prompt (default: first loaded model)")
	temperature := flag.Float64("temp", 0.7, "Temperature for sampling (default: 0.7)")
	verbose := flag.Bool("v", false, "Enable verbose logging")
	trace := flag.Bool("vv", false, "Enable trace logging")
	waitForInterrupt := flag.Bool("wait", false, "Wait for Ctrl+C to exit after command execution")
	checkStatus := flag.Bool("status", false, "Check if the LM Studio service is running")
	showVersion := flag.Bool("version", false, "Show version information")
	jsonOutput := flag.Bool("json", false, "Output list commands in JSON format")
	quiet := flag.Bool("q", false, "Quiet mode - suppress all stdout messages except JSON output and errors")
	quietLong := flag.Bool("quiet", false, "Quiet mode - suppress all stdout messages except JSON output and errors")
	loadTimeout := flag.Duration("timeout", 120*time.Second, "Timeout for loading a model")

	// Parse command line flags
	flag.Parse()

	// Set quiet mode if either -q or -quiet is specified
	quietMode = *quiet || *quietLong

	// Show help if requested
	if flag.NFlag() == 0 {
		fmt.Println("LM Studio Models CLI")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  Status:")
		fmt.Println("     --status")
		fmt.Println("\n  List all loaded models (LLM and embeddings):")
		fmt.Println("     --list-loaded")
		fmt.Println("\n  List loaded LLM models only:")
		fmt.Println("     --list-loaded-llms")
		fmt.Println("\n  List loaded embedding models only:")
		fmt.Println("     --list-loaded-embeddings")
		fmt.Println("\n  List downloaded models:")
		fmt.Println("     --list-downloaded")
		fmt.Println("\n  Load a model:")
		fmt.Println("     --load=mistral-7b-instruct")
		fmt.Println("\n  Unload a model:")
		fmt.Println("     --unload=mistral-7b-instruct")
		fmt.Println("\n  Unload all loaded models:")
		fmt.Println("     --unload-all")
		fmt.Println("\n  Send a prompt with specified model:")
		fmt.Println("     --model=mistral-7b-instruct --prompt=\"Hello, how are you?\"")
		fmt.Println("\n  Send a prompt with custom temperature:")
		fmt.Println("     --model=mistral-7b-instruct --prompt=\"Hello, how are you?\" --temp=0.8")
		fmt.Println("\n  Send a prompt using first loaded model:")
		fmt.Println("     --prompt=\"Hello, how are you?\"")
		fmt.Println("\n  Custom host and port:")
		fmt.Println("     --host=192.168.1.100 --port=5678")
		fmt.Println("\n  Verbose logging (info level):")
		fmt.Println("     -v")
		fmt.Println("\n  Very verbose logging (debug level):")
		fmt.Println("     -vv")
		os.Exit(0)
	}

	// Validate options
	if *loadModel == "" && *unloadModel != "" && *unloadModel == "help" {
		fmt.Println("Please provide a valid model identifier for unloading.")
		fmt.Println("Run with --list-loaded to see available models.")
		os.Exit(1)
	}

	if *unloadModel == "" && *loadModel != "" && *loadModel == "help" {
		fmt.Println("Please provide a valid model identifier for loading.")
		fmt.Println("Run with --list-downloaded to see available models.")
		os.Exit(1)
	}

	logger := lmstudio.NewLogger(lmstudio.LogLevelInfo)
	if *verbose {
		logger.SetLevel(lmstudio.LogLevelDebug)
	}
	if *trace { // Using verbose for trace level logging too
		logger.SetLevel(lmstudio.LogLevelTrace)
	}

	// Handle the different operations based on flags
	var operation bool

	// Show version information if requested
	if *showVersion {
		operation = true
		fmt.Printf("LM Studio Go CLI version: %s\n", lmstudio.LMStudioGoVersion)
		os.Exit(0)
	}

	// Create an LM Studio client
	// If host or port is not set try to discover the LM Studio server on the local network
	serverAddress := fmt.Sprintf("%s:%d", *host, *port)
	if *host == "" || *port == 0 {
		var err error
		logger.Debug("Host and port not explicitly set, attempting to discover LM Studio server...")
		if serverAddress, err = lmstudio.DiscoverLMStudioServer(*host, *port, logger); err == nil {
			logger.Debug("Discovered LM Studio server at %s", serverAddress)
		} else {
			logger.Error("Could not discover LM Studio server, try to set host and port explicitly")
			os.Exit(1)
		}
	}

	client := lmstudio.NewLMStudioClient(serverAddress, logger)
	defer client.Close()

	// Check if LM Studio service is running
	if *checkStatus {
		operation = true
		running, err := client.CheckStatus()
		if err != nil {
			fmt.Printf("LM Studio service status: ERROR - %v\n", err)
			os.Exit(1)
		}
		if running {
			fmt.Printf("LM Studio service status: RUNNING @ %s\n", serverAddress)
		} else {
			fmt.Println("LM Studio service status: NOT RUNNING")
			os.Exit(1)
		}
	}

	// List all loaded models (LLM and embedding)
	if *listLoaded {
		operation = true
		models, err := client.ListAllLoadedModels()
		if err != nil {
			logger.Error("Failed to list loaded models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Loaded Models", *jsonOutput) // Pass jsonOutput
	}

	// List loaded LLM models
	if *listLoadedLLMs {
		operation = true
		models, err := client.ListLoadedLLMs()
		if err != nil {
			logger.Error("Failed to list loaded LLM models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Loaded LLM Models", *jsonOutput) // Pass jsonOutput
	}

	// List loaded embedding models
	if *listLoadedEmbeddings {
		operation = true
		models, err := client.ListLoadedEmbeddingModels()
		if err != nil {
			logger.Error("Failed to list loaded embedding models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Loaded Embedding Models", *jsonOutput) // Pass jsonOutput
	}

	// List downloaded models
	if *listDownloaded {
		operation = true
		models, err := client.ListDownloadedModels()
		if err != nil {
			logger.Error("Failed to list downloaded models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Downloaded Models", *jsonOutput) // Pass jsonOutput
	}

	// Load a model
	if *loadModel != "" {
		operation = true
		if err := loadModelWithProgress(client, *loadTimeout, *loadModel, logger); err != nil {
			logger.Error("Failed to load model: %v", err)
			os.Exit(1)
		}
	}

	// Unload a model
	if *unloadModel != "" {
		operation = true
		quietPrintf("Unloading model: %s\n", *unloadModel)
		if err := client.UnloadModel(*unloadModel); err != nil {
			// Check if the error is about the model not being found
			if strings.Contains(err.Error(), "No model found that fits the query") {
				quietPrintf("Model %s is not currently loaded. No action needed.\n", *unloadModel)
			} else {
				logger.Error("Failed to unload model: %v", err)
				os.Exit(1)
			}
		} else {
			quietPrintf("Model %s unloaded successfully\n", *unloadModel)
		}
	}

	// Handle unload all models command
	if *unloadAll {
		operation = true
		quietPrintf("Unloading all loaded models...\n")
		err := client.UnloadAllModels()
		if err != nil {
			logger.Error("Failed to unload all models: %v", err)
			os.Exit(1)
		}
		quietPrintf("Unloaded all models successfully\n")
	}

	// Handle prompt (new format with separate model and prompt options)
	if *promptText != "" {
		operation = true

		modelIdentifier := *promptModel

		// If no model specified, use the first loaded model
		if modelIdentifier == "" {
			// Get loaded models
			models, err := client.ListLoadedLLMs()
			if err != nil {
				logger.Error("Failed to get loaded models: %v", err)
				os.Exit(1)
			}

			if len(models) == 0 {
				logger.Error("No models are loaded. Please load a model first with --load=<model>")
				os.Exit(1)
			}

			// Use the first loaded model
			modelIdentifier = models[0].Identifier
			if modelIdentifier == "" {
				modelIdentifier = models[0].ModelKey
			}

			quietPrintf("No model specified, using first loaded model: %s\n", modelIdentifier)
		}

		quietPrintf("\nSending prompt to model: %s, temperature: %.2f\n", modelIdentifier, *temperature)
		quietPrintf("Prompt: %s\n", *promptText)
		quietPrintln("Response:")

		// Create a callback to print tokens as they arrive
		callback := func(token string) {
			// Always print tokens regardless of quiet mode - this is the actual output
			fmt.Print(token)
		}

		if err := client.SendPrompt(modelIdentifier, *promptText, *temperature, callback); err != nil {
			logger.Error("Failed to send prompt: %v", err)
			os.Exit(1)
		}
		quietPrintln("") // Print newline only in non-quiet mode
	}

	// If no operation was specified, list all loaded models as the default behavior
	if !operation {
		models, err := client.ListAllLoadedModels()
		if err != nil {
			logger.Error("Failed to list loaded models: %v", err)
			// Decide if os.Exit(1) is appropriate here or just log
		}
		// Only print if there was no error or if models is not nil
		if models != nil {
			printModels(models, "Loaded Models", *jsonOutput) // Pass jsonOutput
		}
	}

	// Wait for Ctrl+C to exit if requested
	if *waitForInterrupt {
		quietPrintln("\nPress Ctrl+C to exit")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		quietPrintln("\nShutting down...")
	}

	// Write coverage data if running instrumented build
	if coverageFile != "" {
		fmt.Printf("Writing coverage data to: %s\n", coverageFile)
		// The actual writing of coverage data happens automatically when the
		// instrumented binary exits, as it's handled by the Go runtime
	}
}
