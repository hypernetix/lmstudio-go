package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/hypernetix/lmstudio-go/pkg/lmstudio"
)

// coverageFile is set at build time via -ldflags for instrumented builds
var coverageFile string

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
		fmt.Printf("%-*s", widths[i], col)
		if i < len(columns)-1 {
			fmt.Printf(" | ")
		}
	}
	fmt.Println()

	// Print separator
	totalWidth := 0
	for i, width := range widths {
		for j := 0; j < width; j++ {
			fmt.Print("-")
		}
		if i < len(widths)-1 {
			fmt.Print("-+-")
			totalWidth += width + 3
		} else {
			totalWidth += width
		}
	}
	fmt.Println()
}

// printModels prints models in a nice table format
func printModels(models []lmstudio.Model, title string) {
	fmt.Printf("\n%s:\n", title)
	if len(models) == 0 {
		fmt.Printf("No %s found\n", strings.ToLower(title))
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
		fmt.Printf("%-*s | %-15s | %-10s | %-10s | %-10s | %-50s\n",
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

func main() {
	// Setup code coverage if running instrumented build
	if coverageFile != "" {
		fmt.Printf("Running with code coverage. Data will be written to: %s\n", coverageFile)
	}

	// Define command-line flags
	host := flag.String("host", "localhost", "LM Studio API host (default: localhost)")
	port := flag.Int("port", 1234, "LM Studio API port (default: 1234)")
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

	// Parse command line flags
	flag.Parse()

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

	// Create an LM Studio client
	client := lmstudio.NewLMStudioClient(fmt.Sprintf("%s:%d", *host, *port), logger)
	defer client.Close()

	// Handle the different operations based on flags
	var operation bool

	// Show version information if requested
	if *showVersion {
		operation = true
		fmt.Println("LM Studio Go CLI version: 1.0")
		os.Exit(0)
	}

	// Check if LM Studio service is running
	if *checkStatus {
		operation = true
		running, err := client.CheckStatus()
		if err != nil {
			fmt.Printf("LM Studio service status: ERROR - %v\n", err)
			os.Exit(1)
		}
		if running {
			fmt.Printf("LM Studio service status: RUNNING @ %s:%d\n", *host, *port)
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
		printModels(models, "Loaded Models")
	}

	// List loaded LLM models
	if *listLoadedLLMs {
		operation = true
		models, err := client.ListLoadedLLMs()
		if err != nil {
			logger.Error("Failed to list loaded LLM models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Loaded LLM Models")
	}

	// List loaded embedding models
	if *listLoadedEmbeddings {
		operation = true
		models, err := client.ListLoadedEmbeddingModels()
		if err != nil {
			logger.Error("Failed to list loaded embedding models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Loaded Embedding Models")
	}

	// List downloaded models
	if *listDownloaded {
		operation = true
		models, err := client.ListDownloadedModels()
		if err != nil {
			logger.Error("Failed to list downloaded models: %v", err)
			os.Exit(1)
		}
		printModels(models, "Downloaded Models")
	}

	// Load a model
	if *loadModel != "" {
		operation = true
		fmt.Printf("Loading model: %s\n", *loadModel)
		if err := client.LoadModel(*loadModel); err != nil {
			logger.Error("Failed to load model: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Model %s loaded successfully\n", *loadModel)
	}

	// Unload a model
	if *unloadModel != "" {
		operation = true
		fmt.Printf("Unloading model: %s\n", *unloadModel)
		if err := client.UnloadModel(*unloadModel); err != nil {
			// Check if the error is about the model not being found
			if strings.Contains(err.Error(), "No model found that fits the query") {
				fmt.Printf("Model %s is not currently loaded. No action needed.\n", *unloadModel)
			} else {
				logger.Error("Failed to unload model: %v", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Model %s unloaded successfully\n", *unloadModel)
		}
	}

	// Handle unload all models command
	if *unloadAll {
		operation = true
		fmt.Printf("Unloading all loaded models...\n")
		err := client.UnloadAllModels()
		if err != nil {
			logger.Error("Failed to unload all models: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Unloaded all models successfully\n")
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

			fmt.Printf("No model specified, using first loaded model: %s\n", modelIdentifier)
		}

		fmt.Printf("\nSending prompt to model: %s, temperature: %.2f\n", modelIdentifier, *temperature)
		fmt.Printf("Prompt: %s\n", *promptText)
		fmt.Println("Response:")

		// Create a callback to print tokens as they arrive
		callback := func(token string) {
			fmt.Print(token)
		}

		if err := client.SendPrompt(modelIdentifier, *promptText, *temperature, callback); err != nil {
			logger.Error("Failed to send prompt: %v", err)
			os.Exit(1)
		}
		fmt.Println("")
	}

	// If no operation was specified, list all loaded models as the default behavior
	if !operation {
		models, err := client.ListAllLoadedModels()
		if err != nil {
			logger.Error("Failed to list loaded models: %v", err)
		}
		printModels(models, "Loaded Models")
	}

	// Wait for Ctrl+C to exit if requested
	if *waitForInterrupt {
		fmt.Println("\nPress Ctrl+C to exit")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("\nShutting down...")
	}

	// Write coverage data if running instrumented build
	if coverageFile != "" {
		fmt.Printf("Writing coverage data to: %s\n", coverageFile)
		// The actual writing of coverage data happens automatically when the
		// instrumented binary exits, as it's handled by the Go runtime
	}
}
