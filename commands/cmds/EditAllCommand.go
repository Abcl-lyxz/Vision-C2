package cmds

import (
	"arismcnc/database"
	"arismcnc/utils"
	"fmt"
	"io"
	"strconv"

	"github.com/gliderlabs/ssh"
)

type EditAllCommand struct{}

func (c *EditAllCommand) Name() string {
	return "editall"
}

func (c *EditAllCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	// Ask the user for the operation on the same line
	fmt.Fprintf(output, "Select what you want to do (add_days): ")
	operation, err := utils.ReadLine(session)
	if err != nil {
		fmt.Fprintln(output, "Error reading input:", err)
		return
	}

	switch operation {
	case "add_days":
		// Ask for the number of days on the same line
		fmt.Fprintf(output, "How many days do you want to add: ")
		daysInput, err := utils.ReadLine(session)
		if err != nil {
			fmt.Fprintln(output, "Error reading days input:", err)
			return
		}

		// Parse days input
		days, err := strconv.Atoi(daysInput)
		if err != nil {
			fmt.Fprintln(output, "Invalid number of days.")
			return
		}

		// Execute the operation (e.g., add days to all users' expiry)
		err = db.AddDaysEveryone(days)
		if err != nil {
			fmt.Fprintln(output, "Failed to add days to users:", err)
			return
		}
		fmt.Fprintf(output, "Successfully added %d days to all users.\n", days)

	default:
		fmt.Fprintln(output, "Invalid option. Available options: add_days")
	}
}

func (c *EditAllCommand) AdminOnly() bool {
	return true
}

func (c *EditAllCommand) Aliases() []string {
	return []string{"editall", "editsall"}
}

// Register command in CommandMap
func init() {
	CommandMap["editall"] = &EditAllCommand{}
}
