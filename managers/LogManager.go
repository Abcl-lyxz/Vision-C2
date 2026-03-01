package managers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

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

type LogManager struct {
	config  LogConfig
	logFile *os.File
}

func NewLogManager(configPath string) (*LogManager, error) {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config LogConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var logFile *os.File
	if config.Global.Enabled && config.Global.LogInFiles {
		logFile, err = os.OpenFile("./assets/logs/global_logs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
	}

	return &LogManager{config: config, logFile: logFile}, nil
}

func (lm *LogManager) Log(message string) {
	if lm.config.Global.Enabled {
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
}

func (lm *LogManager) sendToTelegram(message string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", lm.config.Telegram.BotToken)
	payload := map[string]interface{}{
		"chat_id": lm.config.Telegram.ChatID,
		"text":    message,
	}
	data, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to send Telegram message: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (lm *LogManager) sendToDiscord(message string) {
	payload := map[string]interface{}{
		"content": message,
	}
	data, _ := json.Marshal(payload)
	resp, err := http.Post(lm.config.Discord.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("failed to send Discord message: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (lm *LogManager) Close() {
	if lm.logFile != nil {
		lm.logFile.Close()
	}
}
