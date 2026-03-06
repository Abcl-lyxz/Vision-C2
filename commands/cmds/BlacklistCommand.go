package cmds

import (
	"fmt"
	"io"
	"net"
	"strings"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// BlacklistCommand lets admins manage the IP blacklist from the panel
type BlacklistCommand struct{}

func (c *BlacklistCommand) Name() string        { return "blacklist" }
func (c *BlacklistCommand) AdminOnly() bool      { return true }
func (c *BlacklistCommand) Aliases() []string    { return []string{"bl"} }

func (c *BlacklistCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	const path = "assets/blacklists/list.json"
	theme := utils.GetTheme()

	if len(args) == 0 {
		fmt.Fprintln(output, "Usage: blacklist <list|add|remove> [ip/cidr]")
		return
	}

	switch strings.ToLower(args[0]) {
	case "list":
		entries := utils.ReadBlacklistedIPs(path)
		if len(entries) == 0 {
			fmt.Fprintln(output, theme.Colors.Warning+"Blacklist is empty."+theme.Colors.Reset)
			return
		}
		fmt.Fprintf(output, "%sBlacklist (%d entries):%s\n", theme.Colors.Info, len(entries), theme.Colors.Reset)
		for i, e := range entries {
			fmt.Fprintf(output, "  %d. %s\n", i+1, e)
		}

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: blacklist add <ip/cidr>")
			return
		}
		entry := args[1]
		if net.ParseIP(entry) == nil {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				fmt.Fprintf(output, "%sInvalid IP or CIDR: %s%s\n", theme.Colors.Error, entry, theme.Colors.Reset)
				return
			}
		}
		entries := utils.ReadBlacklistedIPs(path)
		for _, e := range entries {
			if e == entry {
				fmt.Fprintf(output, "%s%s is already blacklisted.%s\n", theme.Colors.Warning, entry, theme.Colors.Reset)
				return
			}
		}
		entries = append(entries, entry)
		utils.EditBlacklistedIPs(path, entries)
		fmt.Fprintf(output, "%sAdded %s to blacklist.%s\n", theme.Colors.Success, entry, theme.Colors.Reset)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: blacklist remove <ip/cidr>")
			return
		}
		entry := args[1]
		entries := utils.ReadBlacklistedIPs(path)
		newList := entries[:0]
		found := false
		for _, e := range entries {
			if e == entry {
				found = true
			} else {
				newList = append(newList, e)
			}
		}
		if !found {
			fmt.Fprintf(output, "%s%s not found in blacklist.%s\n", theme.Colors.Warning, entry, theme.Colors.Reset)
			return
		}
		utils.EditBlacklistedIPs(path, newList)
		fmt.Fprintf(output, "%sRemoved %s from blacklist.%s\n", theme.Colors.Success, entry, theme.Colors.Reset)

	default:
		fmt.Fprintln(output, "Usage: blacklist <list|add|remove> [ip/cidr]")
	}
}

func init() {
	CommandMap["blacklist"] = &BlacklistCommand{}
}
