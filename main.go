package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"arismcnc/database"
	"arismcnc/handlers"
	"arismcnc/utils"

	"github.com/gliderlabs/ssh"
)

func main() {
	// Load configuration
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("\033[32mSuccessfully\033[0m loaded config (assets/config.json)")

	// Load theme
	utils.LoadTheme("assets/theme.json")
	log.Println("\033[32mSuccessfully\033[0m loaded theme (assets/theme.json)")

	// Connect to database
	db, err := database.ConnectDB(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("\033[32mSuccessfully\033[0m connected to database (" + config.DBHost + ":3306)")
	defer db.DB.Close()

	// Setup database schema
	if err := database.SetupDatabaseSchema(db.DB); err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}
	log.Println("\033[32mSuccessfully\033[0m completed database schema setup")

	// Create default root user if no users exist
	if err := database.CreateDefaultUser(db.DB); err != nil {
		log.Fatalf("Failed to create default user: %v", err)
	}

	var wg sync.WaitGroup

	// Start HTTP API server
	wg.Add(1)
	go func() {
		defer wg.Done()
		handlers.StartHTTPServer()
	}()

	// Setup SSH server
	sshServer := ssh.Server{
		Addr: ":" + config.Port,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return db.AuthenticateUser(ctx.User(), password)
		},
		Handler: func(session ssh.Session) {
			handlers.SessionHandler(db, session)
		},
	}

	publicIP, err := utils.GetPublicIP()
	if err != nil {
		log.Printf("Error getting public IP address: %v", err)
	}
	log.Printf("\033[32mSuccessfully\033[0m started SSH server (%s:%s)", strings.TrimSpace(publicIP), config.Port)

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("Received %v, shutting down gracefully...", sig)
		sshServer.Shutdown(context.Background())
		db.DB.Close()
		os.Exit(0)
	}()

	if err := sshServer.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
		log.Fatalf("Failed to start SSH server: %v", err)
	}
}
