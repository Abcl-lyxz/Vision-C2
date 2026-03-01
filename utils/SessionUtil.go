package utils

import (
	"strings"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// SendMessage writes a message to the SSH session with optional newline
func SendMessage(session ssh.Session, message string, newline bool) {
	if newline {
		session.Write([]byte(message + "\r\n"))
	} else {
		session.Write([]byte(message))
	}
}

// SetTitle sets the terminal window title via ANSI escape
func SetTitle(session ssh.Session, message string) {
	session.Write([]byte("\033]0;" + message + "\007"))
}

// ReadLine reads a line of input from the SSH session
func ReadLine(session ssh.Session) (string, error) {
	terminal := terminal.NewTerminal(session, "")
	input, err := terminal.ReadLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// GenerateRoleLabels creates colored role badges from theme config
func GenerateRoleLabels(isAdmin, isVip, isPrivate int) string {
	theme := GetTheme()
	roles := ""

	if isAdmin == 1 {
		roles += theme.Colors.RoleAdmin + " A " + theme.Colors.Reset + " "
	}
	if isVip == 1 {
		roles += theme.Colors.RoleVip + " V " + theme.Colors.Reset + " "
	}
	if isPrivate == 1 {
		roles += theme.Colors.RolePrivate + " P " + theme.Colors.Reset + " "
	}

	if roles == "" {
		return theme.Colors.RoleUser + " U " + theme.Colors.Reset + " "
	}
	return roles
}
