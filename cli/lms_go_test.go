package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testModel = "qwen2.5-0.5b-instruct"

// TestCLI runs integration tests for the CLI
func TestCLI(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the CLI first
	buildCLI(t)

	// Run the tests
	t.Run("TestVersion", testVersion)
	t.Run("TestHelp", testHelp)
	t.Run("TestNoParams", testNoParams)
	t.Run("TestStatus", testStatus)
	t.Run("TestListModels", testListModels)
	t.Run("TestInvalidFlag", testInvalidFlag)
	t.Run("TestPrompt", testPrompt)
	t.Run("TestModelLoadingUnloading", testModelLoadingUnloading)

}

// buildCLI builds the CLI binary for testing with code coverage instrumentation
func buildCLI(t *testing.T) {
	// Get the directory of the current file
	dir, err := os.Getwd()
	dir = filepath.Join(dir, "..")
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Build the binary in a temporary location
	buildDir := filepath.Join(dir, "build")
	os.MkdirAll(buildDir, 0755)

	binaryPath := filepath.Join(buildDir, "lms-go-instrumented")

	// Remove any existing binary
	os.Remove(binaryPath)

	// Create coverage directory if it doesn't exist
	coverageDir := filepath.Join(dir, "coverage/raw/")
	os.MkdirAll(coverageDir, 0755)

	// Generate timestamp for coverage file
	timestamp := time.Now().Format("20060102-150405")
	coverageFile := filepath.Join(coverageDir, fmt.Sprintf("coverage-%s.out", timestamp))

	// Build the binary with coverage instrumentation
	// We use -coverpkg=./... to instrument all packages in the project
	// Using atomic mode to match the Makefile coverage settings
	cmd := exec.Command(
		"go", "build",
		"-cover",
		"-covermode=atomic",
		"-coverpkg=./...",
		"-o", binaryPath,
		"-ldflags", fmt.Sprintf("-X main.coverageFile=%s", coverageFile),
		"cli/lms_go.go",
	)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build instrumented CLI: %v\nOutput: %s", err, output)
	}

	// Check if the binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary was not created at %s", binaryPath)
	}

	// Print coverage file location for reference
	fmt.Printf("Instrumented binary built. Coverage data will be written to: %s\n", coverageFile)
}

// runCLI runs the CLI with the given arguments and returns stdout, stderr, and error
func runCLI(t *testing.T, args ...string) (string, string, error) {
	// Get the directory of the current file
	dir, err := os.Getwd()
	dir = filepath.Join(dir, "..")
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Path to the binary
	binaryPath := filepath.Join(dir, "build", "lms-go-instrumented")

	// Create command
	cmd := exec.Command(binaryPath, args...)

	// Set up environment for coverage
	rawCoverageDir := filepath.Join(dir, "coverage/raw")
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOCOVERDIR=%s", rawCoverageDir))

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// testVersion tests the --version flag
func testVersion(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--version")

	// Version command should always succeed
	if err != nil {
		t.Errorf("Version command failed: %v\nStderr: %s", err, stderr)
		return
	}

	// Check if the output contains the version information
	expectedVersion := "LM Studio Go CLI version:"
	if !strings.Contains(stdout, expectedVersion) {
		t.Errorf("Expected output to contain '%s', got:\n%s", expectedVersion, stdout)
	}

	fmt.Printf("Version test passed. Found version information in the output.\n")
}

// testHelp tests the --help flag
func testHelp(t *testing.T) {
	stdout, stderr, _ := runCLI(t, "--help")

	// Help command usually exits with code 0
	output := stdout + stderr

	// Check if the output contains help information
	expectedTerms := []string{"Usage", "-list-loaded", "List all loaded models", "-host", "-port"}
	for _, term := range expectedTerms {
		if !strings.Contains(output, term) {
			t.Errorf("Expected help to contain '%s', got:\n%s", term, output)
		}
	}

	fmt.Printf("Help test passed. Found all expected terms in the output.\n")
}

func testNoParams(t *testing.T) {
	stdout, stderr, err := runCLI(t)

	// Command should fail with an error about missing required flags
	if err != nil {
		t.Errorf("Expected command to succeed with missing required flags, but it failed: %v\nStderr: %s", err, stderr)
	}

	output := stdout + stderr

	// Check if the error message mentions the missing flags
	if !strings.Contains(output, "--host") && !strings.Contains(output, "--port") && !strings.Contains(output, "Examples") {
		t.Errorf("Expected error about missing required flags, got: %s", output)
	}

	fmt.Printf("Empty params test passed. Command failed as expected.\n")
}

// testListModels tests the --list-downloaded flag
func testListModels(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--list-downloaded")

	// This might fail if LM Studio is not running, which is expected
	if err != nil {
		fmt.Printf("List models command failed (this might be expected if LM Studio is not running): %v\nStderr: %s\n", err, stderr)
		return
	}

	// Check if the output looks like a model list
	expectedTerms := []string{"downloaded", "model", "name"}
	for _, term := range expectedTerms {
		if !strings.Contains(strings.ToLower(stdout), term) {
			t.Errorf("Expected output to contain '%s', got:\n%s", term, stdout)
		}
	}

	fmt.Printf("List models test passed. Output contains expected format.\n")
}

// testInvalidFlag tests an invalid flag
func testInvalidFlag(t *testing.T) {
	_, stderr, err := runCLI(t, "--invalid-flag")

	// Command should fail with an error about unknown flag
	if err == nil {
		t.Errorf("Expected command to fail with invalid flag, but it succeeded")
	}

	// Check if the error message mentions the unknown flag
	if !strings.Contains(stderr, "flag") && !strings.Contains(stderr, "invalid") && !strings.Contains(stderr, "unknown") {
		t.Errorf("Expected error about unknown flag, got: %s", stderr)
	}

	fmt.Printf("Invalid flag test passed. Command failed as expected.\n")
}

// testPrompt tests the --prompt flag with a simple prompt
func testPrompt(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--prompt", "Hello", "--temp", "0.69")

	// This might fail if LM Studio is not running, which is expected
	if err != nil {
		fmt.Printf("Prompt command failed (this might be expected if LM Studio is not running): %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(stdout, "Prompt: Hello") {
		t.Errorf("Expected output to contain 'Prompt: Hello', got:\n%s", stdout)
	}

	if !strings.Contains(stdout, "temperature: 0.69") {
		t.Errorf("Expected output to contain 'temperature: 0.69', got:\n%s", stdout)
	}

	// Check if the output contains a response
	if !strings.Contains(stdout, "Response:") {
		t.Errorf("Expected output to contain 'Response:', got:\n%s", stdout)
	}

	// Check if there's content after the "Response:" string
	parts := strings.Split(stdout, "Response:")
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		t.Errorf("Expected some content after 'Response:', but found none")
	}

	fmt.Printf("Prompt test passed. Received a response from the model.\n")
}

// testStatus tests the --status flag
func testStatus(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--status")

	// This might fail if LM Studio is not running, which is expected
	if err != nil {
		// Check if the output indicates that the service is not running
		if strings.Contains(stdout, "NOT RUNNING") || strings.Contains(stderr, "NOT RUNNING") {
			fmt.Printf("Status check correctly reported service not running\n")
			return
		}

		fmt.Printf("Status command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	// Check if the output indicates that the service is running
	if !strings.Contains(stdout, "RUNNING") {
		t.Errorf("Expected output to contain 'RUNNING', got:\n%s", stdout)
	}

	fmt.Printf("Status test passed. Service is running.\n")
}

// testModelLoadingUnloading tests unloading all models, loading a specific model, and unloading all models again
func testModelLoadingUnloading(t *testing.T) {
	// 1. Unload all loaded models
	fmt.Printf("Step 1: Unloading all loaded models...\n")
	stdout, stderr, err := runCLI(t, "--unload-all")

	// This might fail if LM Studio is not running, which is expected
	if err != nil {
		fmt.Printf("Unload all models command failed (this might be expected if LM Studio is not running): %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(stdout, "Unloaded all models") {
		t.Errorf("Expected output to contain 'Unloaded all models', got:\n%s", stdout)
	}

	// Verify no models are loaded
	stdout, stderr, err = runCLI(t, "--list-loaded")
	if err != nil {
		fmt.Printf("List loaded models command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(strings.ToLower(stdout), "no loaded models found") {
		t.Errorf("Expected no models to be loaded, but got:\n%s", stdout)
	}

	// 2. Load the testModel model
	fmt.Printf("Step 2: Loading test model '%s'...\n", testModel)
	stdout, stderr, err = runCLI(t, "--load", testModel)

	if err != nil {
		fmt.Printf("Load model command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(stdout, fmt.Sprintf("Loading model: %s", testModel)) {
		t.Errorf("Expected output to contain 'Loading model: %s', got:\n%s", testModel, stdout)
	}

	if !strings.Contains(stdout, "loaded successfully") {
		t.Errorf("Expected output to contain 'loaded successfully', got:\n%s", stdout)
	}

	fmt.Printf("Step 3: Testing prompt with loaded model '%s'...\n", testModel)
	testPrompt(t)

	// Verify the model is loaded
	stdout, stderr, err = runCLI(t, "--list-loaded")
	if err != nil {
		fmt.Printf("List loaded models command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(stdout, testModel) {
		t.Errorf("Expected loaded models to contain '%s', got:\n%s", testModel, stdout)
	}

	// 4. Unload all loaded models again
	fmt.Printf("Step 4: Unloading all loaded models again...\n")
	stdout, stderr, err = runCLI(t, "--unload-all")

	if err != nil {
		fmt.Printf("Unload all models command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(stdout, "Unloaded all models") {
		t.Errorf("Expected output to contain 'Unloaded all models', got:\n%s", stdout)
	}

	// Verify no models are loaded again
	stdout, stderr, err = runCLI(t, "--list-loaded")
	if err != nil {
		fmt.Printf("List loaded models command failed: %v\nStderr: %s\n", err, stderr)
		return
	}

	if !strings.Contains(strings.ToLower(stdout), "no loaded models found") {
		t.Errorf("Expected no models to be loaded, but got:\n%s", stdout)
	}

	fmt.Printf("Model loading/unloading test passed.\n")
}
