package utils

import (
	"strconv"
	"strings"
)

// ReplacePlaceholders substitutes all known placeholder variants in an API URL.
// Supports both {host}/{port}/{time}/{method} and <<$host>>/<<$port>>/<<$time>>/<<$method>> syntaxes.
func ReplacePlaceholders(apiURL, method, target, port string, duration uint32) string {
	timeStr := strconv.Itoa(int(duration))
	r := strings.NewReplacer(
		"{host}", target,
		"{port}", port,
		"{time}", timeStr,
		"{method}", method,
		"<<$host>>", target,
		"<<$port>>", port,
		"<<$time>>", timeStr,
		"<<$method>>", method,
	)
	return r.Replace(apiURL)
}

// ReplacePlaceholdersFunnel substitutes placeholders in funnel API URLs,
// additionally supporting {username} and {password} tokens.
func ReplacePlaceholdersFunnel(apiURL, username, password, target, port, timeStr, method string) string {
	r := strings.NewReplacer(
		"{host}", target,
		"{port}", port,
		"{time}", timeStr,
		"{method}", method,
		"{username}", username,
		"{password}", password,
		"<<$host>>", target,
		"<<$port>>", port,
		"<<$time>>", timeStr,
		"<<$method>>", method,
		"<<$username>>", username,
		"<<$password>>", password,
	)
	return r.Replace(apiURL)
}
