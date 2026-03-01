package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	License         string `json:"-"`
	Port            string `json:"-"`
	Funnel_port     string `json:"-"`
	Attacks_enabled bool   `json:"-"`
	Global_cooldown int    `json:"-"`
	Global_slots    int    `json:"-"`
	DBUser          string `json:"-"`
	DBPass          string `json:"-"`
	DBHost          string `json:"-"`
	DBName          string `json:"-"`
}

type AuxConfig struct {
	CNC struct {
		License         string `json:"license"`
		Port            string `json:"port"`
		Funnel_port     string `json:"api_port"`
		Attacks_enabled bool   `json:"attacks_enabled"`
		Global_cooldown int    `json:"global_cooldown"`
		Global_slots    int    `json:"global_slots"`
	} `json:"cnc"`
	MySQL struct {
		DBUser string `json:"db_user"`
		DBPass string `json:"db_pass"`
		DBHost string `json:"db_host"`
		DBName string `json:"db_name"`
	} `json:"mysql"`
}

// UnmarshalJSON has been kept as is
func (c *Config) UnmarshalJSON(data []byte) error {
	aux := AuxConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Mapping data from aux to our Config
	c.License = aux.CNC.License
	c.Port = aux.CNC.Port
	c.Funnel_port = aux.CNC.Funnel_port
	c.Attacks_enabled = aux.CNC.Attacks_enabled
	c.Global_cooldown = aux.CNC.Global_cooldown
	c.Global_slots = aux.CNC.Global_slots
	c.DBUser = aux.MySQL.DBUser
	c.DBPass = aux.MySQL.DBPass
	c.DBHost = aux.MySQL.DBHost
	c.DBName = aux.MySQL.DBName

	return nil
}

// / LoadConfig loads the config from the JSON file
func LoadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ToggleAttacks toggles the attacks_enabled flag
func (c *Config) ToggleAttacks() error {
	// Toggle the Attacks_enabled flag
	c.Attacks_enabled = !c.Attacks_enabled

	// Save only the changes to config file
	return c.SaveConfig("assets/config.json")
}

// SaveConfig saves only the modified config to the JSON file
func (c *Config) SaveConfig(filePath string) error {
	// Open the config file for reading and writing
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening config file for writing: %v", err)
	}
	defer file.Close()

	// Load the full configuration to preserve data
	var auxConfig AuxConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&auxConfig); err != nil {
		return fmt.Errorf("error decoding config: %v", err)
	}

	// Update the attack_enabled flag in the config data
	auxConfig.CNC.Attacks_enabled = c.Attacks_enabled

	// Re-open the file for writing (truncate the file content)
	file.Close()
	file, err = os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening config file for writing: %v", err)
	}
	defer file.Close()

	// Create a JSON encoder to write the struct back to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print

	// Write the updated config data back to the file
	if err := encoder.Encode(auxConfig); err != nil {
		return fmt.Errorf("error saving config file: %v", err)
	}

	return nil
}
