package cmds

import (
	"fmt"
	"io"
	"visioncnc/database"
	"visioncnc/managers"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// ReloadCommand allows admins to reload config, methods, and theme without restart
type ReloadCommand struct{}

func (c *ReloadCommand) Name() string {
	return "reload"
}

func (c *ReloadCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	// Reload config
	_, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		fmt.Fprintln(output, "Failed to reload config:", err)
		return
	}

	// Reload methods/funnel
	utils.ReloadMethods()

	// Reload theme
	utils.LoadTheme("assets/theme.json")

	// Reload branding
	utils.Init()

	// Reload log manager
	managers.ReloadSharedLogManager()

	fmt.Fprintln(output, "Config, methods, theme, and branding reloaded successfully.")
}

func (c *ReloadCommand) AdminOnly() bool {
	return true
}

func (c *ReloadCommand) Aliases() []string {
	return []string{"reload"}
}

func init() {
	CommandMap["reload"] = &ReloadCommand{}
}
