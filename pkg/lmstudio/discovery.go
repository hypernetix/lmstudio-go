package lmstudio

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

func generateUrls(hosts []string, ports []int) (urls []string) {

	protocols := []string{"http", "https"}

	if len(hosts) == 0 || hosts[0] == "" {
		hosts = LMStudioAPIHosts
	}

	if len(ports) == 0 || ports[0] == 0 {
		ports = LMStudioAPIPorts
	}

	for _, proto := range protocols {
		for _, host := range hosts {
			for _, port := range ports {
				urls = append(urls, fmt.Sprintf("%s://%s:%d", proto, host, port))
			}
		}
	}
	return urls
}

// DiscoverLMStudioServer attempts to discover an LM Studio server running on the local network.
// It first checks if the server is running on localhost, and if not, it tries to find it on the local network interfaces.
// Returns the discovered LMStudioAddr if found, or an error if not found.
func DiscoverLMStudioServer(host string, port int, logger Logger) (discoveredUrl string, err error) {

	if logger == nil {
		logger = NewLogger(LogLevelInfo)
	}

	logger.Debug("Attempting to discover LM Studio server...")

	localhostUrls := generateUrls([]string{host}, []int{port})

	for _, url := range localhostUrls {
		logger.Debug("Checking if LM Studio server is running at %s", url)
		if isServerRunning(url, logger) {
			logger.Debug("LM Studio server found at %s", url)
			return url, nil
		}
	}

	// If not found on localhost, check all network interfaces
	netAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	ipaddrs := []string{}

	for _, netAddr := range netAddrs {
		if ipnet, ok := netAddr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			// Found a non-loopback IPv4 address
			ipAddr := ipnet.IP.String()
			ipaddrs = append(ipaddrs, ipAddr)
		}
	}

	networkUrls := generateUrls(ipaddrs, []int{port})

	for _, url := range networkUrls {
		logger.Debug("Checking if LM Studio server is running at %s", url)
		if isServerRunning(url, logger) {
			logger.Info("LM Studio server found at %s", url)
			return url, nil
		}
	}

	return "", fmt.Errorf("no LM Studio server found on the local network")
}

// isServerRunning checks if an LM Studio server is running at the given address
func isServerRunning(url string, logger Logger) bool {
	// Set a short timeout for the HTTP client
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	url += "/v1/models"

	// Try to connect to the models.json endpoint which should be available if the server is running
	resp, err := client.Get(url)

	if err != nil {
		logger.Debug("Failed to connect to %s: %v", url, err)
		return false
	}

	defer resp.Body.Close()

	// Check if the response status code is 200 OK
	if resp.StatusCode == http.StatusOK {
		logger.Debug("Successfully connected to LM Studio server at %s", url)
		return true
	}

	logger.Debug("Received unexpected status code %d from %s", resp.StatusCode, url)
	return false
}
