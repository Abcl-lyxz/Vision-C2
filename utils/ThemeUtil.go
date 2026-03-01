package utils

import (
	"encoding/json"
	"os"
)

// Theme holds all UI color and prefix settings loaded from theme.json
type Theme struct {
	Colors struct {
		Success     string `json:"success"`
		Error       string `json:"error"`
		Warning     string `json:"warning"`
		Info        string `json:"info"`
		Reset       string `json:"reset"`
		TableHeader string `json:"table_header"`
		RoleAdmin   string `json:"role_admin"`
		RoleVip     string `json:"role_vip"`
		RolePrivate string `json:"role_private"`
		RoleUser    string `json:"role_user"`
	} `json:"colors"`
	Prefix string `json:"prefix"`
}

var globalTheme *Theme

// LoadTheme loads theme.json, falls back to defaults if file missing
func LoadTheme(filePath string) *Theme {
	theme := &Theme{}
	// Set defaults
	theme.Colors.Success = "\033[32m"
	theme.Colors.Error = "\033[91m"
	theme.Colors.Warning = "\033[93m"
	theme.Colors.Info = "\033[37;1m"
	theme.Colors.Reset = "\033[0m"
	theme.Colors.TableHeader = "\033[37;1m"
	theme.Colors.RoleAdmin = "\033[41;37m"
	theme.Colors.RoleVip = "\033[43;30m"
	theme.Colors.RolePrivate = "\033[44;37m"
	theme.Colors.RoleUser = "\033[47;30m"
	theme.Prefix = "[ Vision ]"

	data, err := os.ReadFile(filePath)
	if err != nil {
		globalTheme = theme
		return theme
	}

	if err := json.Unmarshal(data, theme); err != nil {
		globalTheme = theme
		return theme
	}

	globalTheme = theme
	return theme
}

// GetTheme returns the globally loaded theme
func GetTheme() *Theme {
	if globalTheme == nil {
		return LoadTheme("assets/theme.json")
	}
	return globalTheme
}
