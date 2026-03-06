package cmds

import (
	"fmt"
	"io"
	"strings"
	"time"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// ApisCommand shows API endpoint health status for all methods
type ApisCommand struct{}

func (c *ApisCommand) Name() string    { return "apis" }
func (c *ApisCommand) AdminOnly() bool { return true }
func (c *ApisCommand) Aliases() []string {
	return []string{"!apis"}
}

func (c *ApisCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	statuses := utils.GetAllAPIStatuses()
	theme := utils.GetTheme()

	if len(statuses) == 0 {
		fmt.Fprintln(output, theme.Colors.Warning+"No API endpoints tracked yet. Send an attack first."+theme.Colors.Reset)
		return
	}

	// Table header
	columns := []string{"#", "Label", "Status", "Last OK", "Last Error"}
	maxLens := make([]int, len(columns))
	for i, h := range columns {
		maxLens[i] = len(h)
	}

	rows := make([][]string, len(statuses))
	for i, s := range statuses {
		label := s.Label
		if label == "" {
			label = s.URL
		}
		status := theme.Colors.Success + "OK" + theme.Colors.Reset
		if !s.OK {
			status = theme.Colors.Error + "ERROR" + theme.Colors.Reset
		}
		lastOK := "never"
		if !s.LastOK.IsZero() {
			lastOK = time.Since(s.LastOK).Round(time.Second).String() + " ago"
		}
		lastErr := "never"
		if !s.LastError.IsZero() {
			lastErr = time.Since(s.LastError).Round(time.Second).String() + " ago"
		}
		rows[i] = []string{fmt.Sprintf("%d", i+1), label, status, lastOK, lastErr}
		for j, cell := range rows[i] {
			if j == 2 {
				continue // skip colored status cell for width calc
			}
			if len(cell) > maxLens[j] {
				maxLens[j] = len(cell)
			}
		}
	}

	totalWidths := make([]int, len(columns))
	for i, l := range maxLens {
		totalWidths[i] = l + 2
	}

	sep := func(left, mid, right, fill string) string {
		var b strings.Builder
		b.WriteString(left)
		for i, w := range totalWidths {
			if i > 0 {
				b.WriteString(mid)
			}
			b.WriteString(strings.Repeat(fill, w))
		}
		b.WriteString(right)
		return b.String()
	}

	fmt.Fprintln(output, sep("╔", "╦", "╗", "═"))
	fmt.Fprint(output, "║")
	for i, h := range columns {
		fmt.Fprintf(output, " %-*s ║", maxLens[i], h)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, sep("╠", "╬", "╣", "═"))

	for _, row := range rows {
		fmt.Fprint(output, "║")
		for i, cell := range row {
			if i == 2 {
				// status cell: pad manually (color codes add invisible chars)
				fmt.Fprintf(output, " %s%s ║", cell, strings.Repeat(" ", maxLens[i]-2))
			} else {
				fmt.Fprintf(output, " %-*s ║", maxLens[i], cell)
			}
		}
		fmt.Fprintln(output)
	}

	fmt.Fprintln(output, sep("╚", "╩", "╝", "═"))
}

func init() {
	CommandMap["!apis"] = &ApisCommand{}
}
