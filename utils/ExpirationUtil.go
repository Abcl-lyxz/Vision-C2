package utils

import (
	"fmt"
	"time"
)

// CalculateExpiryString returns a human-readable string for time remaining until expiry
func CalculateExpiryString(expiryTime time.Time) string {
	now := time.Now().UTC()
	nowLocal := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), expiryTime.Location())
	timeUntilExpiry := expiryTime.Sub(nowLocal)

	switch {
	case timeUntilExpiry > 24*time.Hour:
		return fmt.Sprintf("%d days", int(timeUntilExpiry.Hours()/24))
	case timeUntilExpiry > time.Hour:
		return fmt.Sprintf("%d hours", int(timeUntilExpiry.Hours()))
	case timeUntilExpiry > time.Minute:
		return fmt.Sprintf("%d minutes", int(timeUntilExpiry.Minutes()))
	case timeUntilExpiry > time.Second:
		return fmt.Sprintf("%d seconds", int(timeUntilExpiry.Seconds()))
	default:
		return "Expired"
	}
}

// CalculateInt returns a colored "true"/"false" string based on integer value
func CalculateInt(value int) string {
	theme := GetTheme()
	if value == 0 {
		return theme.Colors.Error + "false" + theme.Colors.Reset
	}
	return theme.Colors.Success + "true" + theme.Colors.Reset
}
