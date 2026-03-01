package utils

import (
	"fmt"
	"io"
	"net/http"
)

// GetPublicIP retrieves the public IP address of the server.
func GetPublicIP() (string, error) {
	// Query an external service to get the public IP address
	resp, err := http.Get("http://checkip.amazonaws.com")
	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %v", err)
	}
	defer resp.Body.Close()

	// Ensure the response status is OK
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK response when retrieving IP: %v", resp.StatusCode)
	}

	// Read the response body (IP address)
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read public IP response: %v", err)
	}

	// Return the IP address as a string
	return string(ip), nil
}
