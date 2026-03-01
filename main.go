package main

import (
	"log"
	"strings"
	"sync"

	"arismcnc/database"
	"arismcnc/handlers"
	"arismcnc/utils"

	"github.com/gliderlabs/ssh"
)

func main() {
	// Load configuration
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Fatalf("\033[31mFailed to load config: %v\033[0m", err) // Red text for failure
	} else {
		log.Println("\033[32mSuccessfully\033[0m loaded config (assets/config.json)") // Green "Successfully" text
	}

	// Connect to the database
	db, err := database.ConnectDB(config)
	if err != nil {
		log.Fatalf("\033[31mFailed to connect to database: %v\033[0m", err) // Red text for failure
	} else {
		log.Println("\033[32mSuccessfully\033[0m connected to database (" + config.DBHost + ":3306)") // Green "Successfully" text
	}
	defer db.DB.Close() // Ensure the database connection is closed on shutdown

	// Run the database schema setup
	if err := database.SetupDatabaseSchema(db.DB); err != nil {
		log.Fatalf("\033[31mDatabase setup failed: %v\033[0m", err) // Red text for failure
	} else {
		log.Println("\033[32mSuccessfully\033[0m completed database schema setup") // Green "Successfully" text
	}

	err = database.CreateDefaultUser(db.DB)
	if err != nil {
		log.Fatalf("\033[31mFailed to create default user: %v\033[0m", err) // Red text for failure
	}

	var wg sync.WaitGroup

	// Run HTTP server in a separate goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		handlers.StartHTTPServer()
	}()

	// Set up and start the SSH server
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
		log.Fatalf("\033[31mError getting public IP address: %v\033[0m", err) // Red text for failure
	}
	log.Printf("\033[32mSuccessfully\033[0m started SSH server (%s:%s)", strings.TrimSpace(publicIP), config.Port)

	if err := sshServer.ListenAndServe(); err != nil {
		log.Fatalf("\033[31mFailed to start SSH server: %v\033[0m", err) // Red text for failure
	}
}
