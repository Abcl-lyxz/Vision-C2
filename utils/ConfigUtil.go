package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all runtime configuration loaded from config.json
type Config struct {
	// CNC settings
	Port            string         `json:"-"`
	Funnel_port     string         `json:"-"`
	Attacks_enabled bool           `json:"-"`
	Global_cooldown int            `json:"-"`
	Global_slots    int            `json:"-"`
	Layer_slots     map[string]int `json:"-"` // per-layer slot limits (0 = unlimited)

	// External services
	IpinfoToken string `json:"-"`
	ProxyURL    string `json:"-"`

	// Password policy
	PasswordMinLength int `json:"-"`

	// Idle session timeout (0 = disabled)
	IdleTimeoutMinutes int `json:"-"`

	// Timezone for expiry display
	Timezone string `json:"-"`

	// MySQL settings
	DBUser         string `json:"-"`
	DBPass         string `json:"-"`
	DBHost         string `json:"-"`
	DBName         string `json:"-"`
	DBMaxConns     int    `json:"-"`
	DBConnLifetime int    `json:"-"` // minutes
}

// AuxConfig maps the JSON structure to flat Config fields
type AuxConfig struct {
	CNC struct {
		Port            string         `json:"port"`
		Funnel_port     string         `json:"api_port"`
		Attacks_enabled bool           `json:"attacks_enabled"`
		Global_cooldown int            `json:"global_cooldown"`
		Global_slots    int            `json:"global_slots"`
		Layer_slots        map[string]int `json:"layer_slots"`
		IdleTimeoutMinutes int            `json:"idle_timeout_minutes"`
		IpinfoToken        string         `json:"ipinfo_token"`
		ProxyURL        string         `json:"proxy_url"`
		PasswordMinLen  int            `json:"password_min_length"`
		Timezone        string         `json:"timezone"`
	} `json:"cnc"`
	MySQL struct {
		DBUser         string `json:"db_user"`
		DBPass         string `json:"db_pass"`
		DBHost         string `json:"db_host"`
		DBName         string `json:"db_name"`
		DBMaxConns     int    `json:"max_connections"`
		DBConnLifetime int    `json:"conn_lifetime_minutes"`
	} `json:"mysql"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	aux := AuxConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	c.Port = aux.CNC.Port
	c.Funnel_port = aux.CNC.Funnel_port
	c.Attacks_enabled = aux.CNC.Attacks_enabled
	c.Global_cooldown = aux.CNC.Global_cooldown
	c.Global_slots = aux.CNC.Global_slots
	c.Layer_slots = aux.CNC.Layer_slots
	if c.Layer_slots == nil {
		c.Layer_slots = map[string]int{}
	}
	c.IdleTimeoutMinutes = aux.CNC.IdleTimeoutMinutes
	c.IpinfoToken = aux.CNC.IpinfoToken
	c.ProxyURL = aux.CNC.ProxyURL
	c.PasswordMinLength = aux.CNC.PasswordMinLen
	c.Timezone = aux.CNC.Timezone
	c.DBUser = aux.MySQL.DBUser
	c.DBPass = aux.MySQL.DBPass
	c.DBHost = aux.MySQL.DBHost
	c.DBName = aux.MySQL.DBName
	c.DBMaxConns = aux.MySQL.DBMaxConns
	c.DBConnLifetime = aux.MySQL.DBConnLifetime

	// Defaults
	if c.Timezone == "" {
		c.Timezone = "UTC"
	}
	if c.PasswordMinLength == 0 {
		c.PasswordMinLength = 6
	}
	if c.DBMaxConns == 0 {
		c.DBMaxConns = 25
	}
	if c.DBConnLifetime == 0 {
		c.DBConnLifetime = 5
	}

	return nil
}

// LoadConfig reads and parses the config JSON file
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

// ToggleAttacks flips the attacks_enabled flag and saves
func (c *Config) ToggleAttacks() error {
	c.Attacks_enabled = !c.Attacks_enabled
	return c.SaveConfig("assets/config.json")
}

// SaveConfig writes the current config back to disk preserving JSON structure
func (c *Config) SaveConfig(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening config file for writing: %v", err)
	}
	defer file.Close()

	var auxConfig AuxConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&auxConfig); err != nil {
		return fmt.Errorf("error decoding config: %v", err)
	}

	auxConfig.CNC.Attacks_enabled = c.Attacks_enabled

	file.Close()
	file, err = os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening config file for writing: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(auxConfig); err != nil {
		return fmt.Errorf("error saving config file: %v", err)
	}

	return nil
}
