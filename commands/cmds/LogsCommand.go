package cmds

import (
	"arismcnc/database"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/gliderlabs/ssh"
)

// LogsCommand example command, not restricted to admins
type LogsCommand struct{}

func (c *LogsCommand) Name() string {
	return "logs"
}

func (c *LogsCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	// Display error if no arguments are provided
	if len(args) == 0 {
		fmt.Fprintln(output, "logs list\nlogs clear\nlogs page\u001B[0m")
		return
	}

	// Check which argument was provided
	switch args[0] {
	case "list":
		// Default to displaying page 1
		c.displayLogs(1, session, db, output)
	case "clear":
		// Clear logs
		if db.ClearLogs() {
			fmt.Fprintln(output, "\u001B[92mAll logs have been cleared.\u001B[0m")
		} else {
			fmt.Fprintln(output, "\u001B[91mFailed to clear logs.\u001B[0m")
		}

	default:
		// Check if the provided argument is a page number
		if p, err := strconv.Atoi(args[0]); err == nil && p > 0 {
			c.displayLogs(p, session, db, output) // Display the specified page
		} else {
			fmt.Fprintln(output, "Invalid command. Usage: logs <list|clear|page number>\u001B[0m")
		}
	}
}

func (c *LogsCommand) displayLogs(page int, session ssh.Session, db *database.Database, output io.Writer) {
	// Retrieve all logs
	currentAttacks := db.GetAllAttacks()
	if len(currentAttacks) == 0 {
		fmt.Fprintln(output, "\u001B[91mNo logged attacks.\u001B[0m")
		return
	}

	// Pagination setup - 25 rows per page
	pageSize := 25
	totalPages := (len(currentAttacks) + pageSize - 1) / pageSize

	// Ensure page is within range
	if page > totalPages || page < 1 {
		fmt.Fprintf(output, "Invalid page number. There are only %d pages.\u001B[0m\n", totalPages)
		return
	}

	// Calculate log entries for the current page
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(currentAttacks) {
		end = len(currentAttacks)
	}
	attacksPage := currentAttacks[start:end]

	// Initialize table with tabwriter for alignment
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0) // Brak `AlignRight` wymusza wyrównanie do lewej
	fmt.Fprintf(output, "\u001B[92mAttack logs (Page %d of %d)\u001B[0m\n", page, totalPages)

	// Table header
	fmt.Fprintln(w, "#\tUSERNAME\tTARGET\tPORT\tDURATION\tMETHOD\tWHEN\t")
	fmt.Fprintln(w, "--\t--------\t------\t----\t--------\t------\t----\t")

	// Populate the table rows with data
	for index, attack := range attacksPage {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\t%s\t\n",
			start+index+1, attack.Username, attack.Target, attack.Port, attack.Duration, attack.Method, attack.End)
	}

	// Flush tabwriter to output
	w.Flush()

	// Display navigation message if there are more pages
	if page < totalPages {
		fmt.Fprintln(output, "Type 'logs <page>' to see more or 'logs clear' to clear logs.")
	}
}

func (c *LogsCommand) AdminOnly() bool {
	return true
}

// Aliases for LogsCommand
func (c *LogsCommand) Aliases() []string {
	return []string{"logs", "log"}
}

// Register LogsCommand in the CommandMap
func init() {
	CommandMap["logs"] = &LogsCommand{}
}
