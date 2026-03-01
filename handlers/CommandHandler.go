package handlers

import (
	"fmt"
	"io"
	"strings"

	"arismcnc/commands/cmds"
	"arismcnc/database"
	"arismcnc/managers"
	"arismcnc/utils"

	"github.com/gliderlabs/ssh"
)

// CommandHandler routes user input to registered commands
type CommandHandler struct {
	commands map[string]cmds.Command
	db       *database.Database
	session  ssh.Session
}

// NewCommandHandler initializes CommandHandler with all registered commands
func NewCommandHandler(db *database.Database, session ssh.Session) *CommandHandler {
	handler := &CommandHandler{
		commands: make(map[string]cmds.Command),
		db:       db,
		session:  session,
	}
	handler.loadCommands()
	return handler
}

// loadCommands registers all commands and their aliases
func (ch *CommandHandler) loadCommands() {
	for _, command := range cmds.CommandMap {
		ch.commands[command.Name()] = command
		for _, alias := range command.Aliases() {
			ch.commands[alias] = command
		}
	}
}

// ExecuteCommand checks permissions and runs the matched command
func (ch *CommandHandler) ExecuteCommand(input string, output io.Writer) {
	args := strings.Fields(input)
	if len(args) == 0 {
		return
	}

	commandName := args[0]
	command, exists := ch.commands[commandName]
	if !exists {
		methods := utils.GetMethodsList()
		if !managers.Contains(methods, args[0]) {
			fmt.Fprintf(output, "Unknown command: %s\n", commandName)
		}
		return
	}

	// Admin permission check
	if command.AdminOnly() {
		userInfo := ch.db.GetAccountInfo(ch.session.User())
		if userInfo.Admin != 1 {
			brandingData := BuildBrandingData(ch.session, ch.db)
			msg := utils.Branding(ch.session, "insufficient-permissions", brandingData)
			fmt.Fprintln(output, msg)
			return
		}
	}

	command.Execute(ch.session, ch.db, args[1:], output)
}
