package utils

import (
	"strings"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func SendMessage(session ssh.Session, message string, newline bool) {
	if newline {
		session.Write([]byte(message + "\r\n"))
	} else {
		session.Write([]byte(message))
	}
}

func SetTitle(session ssh.Session, message string) {
	session.Write([]byte("\033]0;" + message + "\007"))
}

func ReadLine(session ssh.Session) (string, error) {
	terminal := terminal.NewTerminal(session, "")
	input, err := terminal.ReadLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func GenerateRoleLabels(isAdmin, isVip, isPrivate int) string {
	roles := ""

	if isAdmin == 1 {
		roles += "\033[41;37m A \033[0m " // Red background, white text for Admin
	}
	if isVip == 1 {
		roles += "\033[43;30m V \033[0m " // Yellow background, black text for VIP
	}
	if isPrivate == 1 {
		roles += "\033[44;37m P \033[0m " // Blue background, white text for Private
	}

	if roles == "" {
		return "\033[47;30m U \033[0m " // Gray background, black text for regular User
	}
	return roles
}
