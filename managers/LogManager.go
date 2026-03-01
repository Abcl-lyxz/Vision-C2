package managers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// LogConfig holds logging configuration for file, Telegram, and Discord
type LogConfig struct {
	Global   GlobalConfig   `json:"global"`
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
}

type GlobalConfig struct {
	Enabled    bool `json:"enabled"`
	LogInFiles bool `json:"log_in_files"`
}

type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type DiscordConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

// LogManager handles multi-channel logging (file, Telegram, Discord)
type LogManager struct {
	config  LogConfig
	logFile *os.File
}

// NewLogManager loads config and opens log file if enabled
func NewLogManager(configPath string) (*LogManager, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config LogConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
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

// Log sends a message to all enabled channels
func (lm *LogManager) Log(message string) {
	if !lm.config.Global.Enabled {
		return
	}

	if lm.config.Global.LogInFiles && lm.logFile != nil {
		log.SetOutput(lm.logFile)
		log.Println(message)
	}

	if lm.config.Telegram.Enabled {
		lm.sendToTelegram(message)
	}
	if lm.config.Discord.Enabled {
		lm.sendToDiscord(message)
	}
}

func (lm *LogManager) sendToTelegram(message string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id": lm.config.Telegram.ChatID,
		"text":    message,
	})
	resp, err := http.Post("https://api.telegram.org/bot"+lm.config.Telegram.BotToken+"/sendMessage", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("failed to send Telegram message: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (lm *LogManager) sendToDiscord(message string) {
	payload, _ := json.Marshal(map[string]interface{}{"content": message})
	resp, err := http.Post(lm.config.Discord.WebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("failed to send Discord message: %v", err)
		return
	}
	defer resp.Body.Close()
}

// Close releases the log file handle
func (lm *LogManager) Close() {
	if lm.logFile != nil {
		lm.logFile.Close()
	}
}
