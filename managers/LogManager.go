package managers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// LogConfig holds logging configuration for file, Telegram, and Discord
type LogConfig struct {
	Global   GlobalConfig    `json:"global"`
	Events   map[string]bool `json:"events"`
	Telegram TelegramConfig  `json:"telegram"`
	Discord  DiscordConfig   `json:"discord"`
}

type GlobalConfig struct {
	Enabled    bool `json:"enabled"`
	LogInFiles bool `json:"log_in_files"`
}

type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	ThreadID string `json:"thread_id"`
}

type DiscordConfig struct {
	Enabled     bool   `json:"enabled"`
	WebhookURL  string `json:"webhook_url"`
	UseEmbeds   bool   `json:"use_embeds"`
	BotUsername string `json:"bot_username"`
	AvatarURL   string `json:"avatar_url"`
}

// LogManager handles multi-channel logging (file, Telegram, Discord)
type LogManager struct {
	config  LogConfig
	logFile *os.File
	fileMu  sync.Mutex
}

// Singleton
var (
	sharedLogMgr *LogManager
	logMgrMu     sync.RWMutex
)

// GetSharedLogManager returns (or lazily initializes) the singleton LogManager.
func GetSharedLogManager() *LogManager {
	logMgrMu.RLock()
	lm := sharedLogMgr
	logMgrMu.RUnlock()
	if lm != nil {
		return lm
	}
	logMgrMu.Lock()
	defer logMgrMu.Unlock()
	if sharedLogMgr != nil {
		return sharedLogMgr
	}
	lm, err := loadLogManager("./assets/logs/logs.json")
	if err != nil {
		log.Printf("Error initializing LogManager: %v", err)
		return nil
	}
	sharedLogMgr = lm
	return sharedLogMgr
}

// ReloadSharedLogManager closes the current singleton and loads a fresh one.
func ReloadSharedLogManager() {
	logMgrMu.Lock()
	defer logMgrMu.Unlock()
	if sharedLogMgr != nil {
		sharedLogMgr.Close()
	}
	lm, err := loadLogManager("./assets/logs/logs.json")
	if err != nil {
		log.Printf("Error reloading LogManager: %v", err)
		sharedLogMgr = nil
		return
	}
	sharedLogMgr = lm
}

// NewLogManager is kept for backward compatibility; prefer GetSharedLogManager.
func NewLogManager(configPath string) (*LogManager, error) {
	return loadLogManager(configPath)
}

func loadLogManager(configPath string) (*LogManager, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config LogConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
	}
	if config.Events == nil {
		config.Events = map[string]bool{}
	}

	var logFile *os.File
	if config.Global.Enabled && config.Global.LogInFiles {
		logFile, err = os.OpenFile("./assets/logs/global_logs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
	}
	return &LogManager{config: config, logFile: logFile}, nil
}

// Close releases the log file handle
func (lm *LogManager) Close() {
	if lm.logFile != nil {
		lm.logFile.Close()
	}
}

// Log sends a plain-text message to all enabled channels (legacy helper).
func (lm *LogManager) Log(message string) {
	if !lm.config.Global.Enabled {
		return
	}
	if lm.config.Global.LogInFiles {
		lm.writeLine(message)
	}
	if lm.config.Telegram.Enabled {
		lm.sendTelegramPlain(message)
	}
	if lm.config.Discord.Enabled {
		lm.sendDiscordPlain(message)
	}
}

// LogEvent sends a structured event to all enabled channels.
// eventType: "attack_sent" | "blacklist_blocked" | "spam_blocked" | "permission_denied"
// fields: key-value pairs (username, target, port, time, method, layer, country, source, …)
func (lm *LogManager) LogEvent(eventType string, fields map[string]string) {
	if !lm.config.Global.Enabled {
		return
	}
	// Event filter
	if enabled, ok := lm.config.Events[eventType]; ok && !enabled {
		return
	}

	plain := buildPlainText(eventType, fields)

	if lm.config.Global.LogInFiles {
		lm.writeLine(plain)
	}
	if lm.config.Telegram.Enabled {
		lm.sendTelegramEvent(eventType, fields)
	}
	if lm.config.Discord.Enabled {
		if lm.config.Discord.UseEmbeds {
			lm.sendDiscordEmbed(eventType, fields)
		} else {
			lm.sendDiscordPlain(plain)
		}
	}
}

// writeLine safely appends a line to the log file.
func (lm *LogManager) writeLine(message string) {
	if lm.logFile == nil {
		return
	}
	lm.fileMu.Lock()
	defer lm.fileMu.Unlock()
	lm.logFile.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + message + "\n")
}

// buildPlainText returns a human-readable log line for any event type.
func buildPlainText(eventType string, fields map[string]string) string {
	title := eventTitle(eventType)
	msg := title
	for _, key := range []string{"source", "username", "target", "port", "time", "method", "layer", "country", "org", "region"} {
		if v, ok := fields[key]; ok && v != "" {
			msg += fmt.Sprintf("\n%s: %s", capitalize(key), v)
		}
	}
	msg += "\n----------------------"
	return msg
}

func eventTitle(eventType string) string {
	switch eventType {
	case "attack_sent":
		return "New Attack!"
	case "blacklist_blocked":
		return "Blacklist Block!"
	case "spam_blocked":
		return "Spam Protection!"
	case "permission_denied":
		return "Permission Denied!"
	default:
		return eventType
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// ── Telegram ──────────────────────────────────────────────────────────────────

func (lm *LogManager) sendTelegramEvent(eventType string, fields map[string]string) {
	icon := eventIcon(eventType)
	source := fields["source"]
	if source == "" {
		source = "C2"
	}
	html := fmt.Sprintf(
		"<b>%s %s (%s)</b>\n\n"+
			"<b>Username:</b> <code>%s</code>\n"+
			"<b>Target:</b> <code>%s</code>\n"+
			"<b>Method:</b> <code>%s</code>  |  <b>Layer:</b> %s\n"+
			"<b>Port:</b> %s  |  <b>Time:</b> %ss\n"+
			"<b>Country:</b> %s",
		icon, eventTitle(eventType), source,
		fields["username"],
		fields["target"],
		fields["method"], fields["layer"],
		fields["port"], fields["time"],
		fields["country"],
	)

	payload := map[string]interface{}{
		"chat_id":                  lm.config.Telegram.ChatID,
		"text":                     html,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	if lm.config.Telegram.ThreadID != "" {
		payload["message_thread_id"] = lm.config.Telegram.ThreadID
	}

	data, _ := json.Marshal(payload)
	resp, err := http.Post(
		"https://api.telegram.org/bot"+lm.config.Telegram.BotToken+"/sendMessage",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		log.Printf("failed to send Telegram message: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (lm *LogManager) sendTelegramPlain(message string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  lm.config.Telegram.ChatID,
		"text":                     message,
		"disable_web_page_preview": true,
	})
	resp, err := http.Post(
		"https://api.telegram.org/bot"+lm.config.Telegram.BotToken+"/sendMessage",
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Printf("failed to send Telegram message: %v", err)
		return
	}
	defer resp.Body.Close()
}

// ── Discord ───────────────────────────────────────────────────────────────────

func (lm *LogManager) sendDiscordEmbed(eventType string, fields map[string]string) {
	color := embedColor(eventType, fields["layer"])
	icon := eventIcon(eventType)
	source := fields["source"]
	if source == "" {
		source = "C2"
	}

	embedFields := []map[string]interface{}{}
	for _, key := range []string{"username", "target", "method", "layer", "port", "time", "country", "org"} {
		if v, ok := fields[key]; ok && v != "" {
			embedFields = append(embedFields, map[string]interface{}{
				"name":   capitalize(key),
				"value":  v,
				"inline": true,
			})
		}
	}

	embed := map[string]interface{}{
		"title":     fmt.Sprintf("%s %s (%s)", icon, eventTitle(eventType), source),
		"color":     color,
		"fields":    embedFields,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body := map[string]interface{}{
		"embeds": []interface{}{embed},
	}
	if lm.config.Discord.BotUsername != "" {
		body["username"] = lm.config.Discord.BotUsername
	}
	if lm.config.Discord.AvatarURL != "" {
		body["avatar_url"] = lm.config.Discord.AvatarURL
	}

	data, _ := json.Marshal(body)
	resp, err := http.Post(lm.config.Discord.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to send Discord embed: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (lm *LogManager) sendDiscordPlain(message string) {
	payload, _ := json.Marshal(map[string]interface{}{"content": message})
	resp, err := http.Post(lm.config.Discord.WebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("failed to send Discord message: %v", err)
		return
	}
	defer resp.Body.Close()
}

// embedColor returns a Discord embed color integer based on event type and layer.
func embedColor(eventType, layer string) int {
	switch eventType {
	case "blacklist_blocked":
		return 0xFFAA00 // orange
	case "spam_blocked":
		return 0xFFDD00 // yellow
	case "permission_denied":
		return 0xFF4444 // red
	default: // attack_sent
		if layer == "LAYER 7" {
			return 0x4488FF // blue
		}
		return 0xFF4444 // red for LAYER 4
	}
}

func eventIcon(eventType string) string {
	switch eventType {
	case "attack_sent":
		return "🔥"
	case "blacklist_blocked":
		return "🚫"
	case "spam_blocked":
		return "⚠️"
	case "permission_denied":
		return "🔒"
	default:
		return "📋"
	}
}
