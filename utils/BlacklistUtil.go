package utils

import (
	"encoding/json"
	"os"
)

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
