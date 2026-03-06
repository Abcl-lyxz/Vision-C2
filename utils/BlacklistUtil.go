package utils

import (
	"encoding/json"
	"net"
	"os"
	"strings"
)

// IsValidTarget returns true if the target is a valid IPv4/IPv6 address or an http/https/www URL.
func IsValidTarget(target string) bool {
	if net.ParseIP(target) != nil {
		return true
	}
	return strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "www.")
}

// IsTargetBlocked returns true if target matches any entry in the blocked list.
// Entries can be exact IPs, CIDR ranges (e.g. "10.0.0.0/8"), or .gov/.edu domain patterns.
func IsTargetBlocked(target string, blockedList []string) bool {
	targetIP := net.ParseIP(target)
	for _, entry := range blockedList {
		if target == entry {
			return true
		}
		// CIDR range check
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			if targetIP != nil && cidr.Contains(targetIP) {
				return true
			}
		}
		// Domain suffix patterns
		if strings.Contains(entry, ".gov") && strings.Contains(target, ".gov") {
			return true
		}
		if strings.Contains(entry, ".edu") && strings.Contains(target, ".edu") {
			return true
		}
	}
	return false
}

func ReadBlacklistedIPs(filename string) []string {
	blacklistedIPs := []string{}
	file, err := os.Open(filename)
	if err != nil {
		return blacklistedIPs
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&blacklistedIPs)
	if err != nil {
		return blacklistedIPs
	}
	return blacklistedIPs
}

func EditBlacklistedIPs(filename string, blacklistedIPs []string) {
	file, err := os.Create(filename)
	if err != nil {
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(blacklistedIPs)
	if err != nil {
		return
	}
}
