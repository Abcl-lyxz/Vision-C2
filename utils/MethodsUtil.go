package utils

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// APIEntry represents a single API endpoint with optional label and weight.
// Supports both plain string and {"url","label","weight"} object in JSON (backward compatible).
type APIEntry struct {
	URL    string
	Label  string
	Weight int
}

func (a *APIEntry) UnmarshalJSON(data []byte) error {
	// Try plain string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.URL = s
		a.Label = s
		a.Weight = 1
		return nil
	}
	// Try object form
	type plain struct {
		URL    string `json:"url"`
		Label  string `json:"label"`
		Weight int    `json:"weight"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	a.URL = p.URL
	a.Label = p.Label
	if a.Label == "" {
		a.Label = a.URL
	}
	if p.Weight <= 0 {
		a.Weight = 1
	} else {
		a.Weight = p.Weight
	}
	return nil
}

// Method represents an attack method configuration from funnel.json
type Method struct {
	Enabled           bool       `json:"enabled"`
	EnabledWithFunnel bool       `json:"enabledWithFunnel"`
	Method            string     `json:"method"`
	Group             string     `json:"group"`
	DefaultPort       uint16     `json:"defaultPort"`
	DefaultTime       uint32     `json:"defaultTime"`
	MinTime           uint32     `json:"minTime"`
	MaxTime           uint32     `json:"maxTime"`
	Permission        []string   `json:"permission"`
	Slots             int        `json:"slots"`
	ApiMode           string     `json:"api_mode"`
	ApiTimeout        int        `json:"api_timeout"`
	ApiRetry          int        `json:"api_retry"`
	API               []APIEntry `json:"API"`
}

var cachedMethods []Method
var methodsLoaded bool

// rrCounters holds atomic round-robin counters per method name
var rrCounters sync.Map

// LoadMethods reads and caches attack methods from funnel.json
func LoadMethods() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	data, err := os.ReadFile(filepath.Join(cwd, "assets/funnel/funnel.json"))
	if err != nil {
		return
	}

	var methods []Method
	if json.Unmarshal(data, &methods) == nil {
		cachedMethods = methods
		methodsLoaded = true
	}
}

// ReloadMethods forces a reload of the methods cache
func ReloadMethods() {
	methodsLoaded = false
	LoadMethods()
}

// GetMethodsList returns cached attack methods, loading them if needed
func GetMethodsList() []Method {
	if !methodsLoaded {
		LoadMethods()
	}
	return cachedMethods
}

// GetMethod finds a method by name
func GetMethod(method string) (Method, error) {
	for _, m := range GetMethodsList() {
		if m.Method == method {
			return m, nil
		}
	}
	return Method{}, errors.New("Method not found")
}

// SelectAPIs returns the subset of API entries to use based on api_mode.
// Modes: "all" (default), "random", "round-robin", "weighted-random"
func SelectAPIs(methodName, mode string, entries []APIEntry) []APIEntry {
	if len(entries) == 0 {
		return entries
	}
	switch mode {
	case "random":
		return []APIEntry{entries[rand.Intn(len(entries))]}

	case "round-robin":
		val, _ := rrCounters.LoadOrStore(methodName, new(int64))
		p := val.(*int64)
		idx := int(atomic.AddInt64(p, 1)-1) % len(entries)
		return []APIEntry{entries[idx]}

	case "weighted-random":
		total := 0
		for _, e := range entries {
			total += e.Weight
		}
		r := rand.Intn(total)
		for _, e := range entries {
			r -= e.Weight
			if r < 0 {
				return []APIEntry{e}
			}
		}
		return []APIEntry{entries[len(entries)-1]}

	default: // "all" or empty string
		return entries
	}
}

// HasVipPermission checks if a method requires VIP access
func HasVipPermission(method string) bool {
	m, err := GetMethod(method)
	if err != nil {
		return false
	}
	for _, p := range m.Permission {
		if strings.ToLower(p) == "vip" {
			return true
		}
	}
	return false
}

// HasPrivatePermission checks if a method requires Private access
func HasPrivatePermission(method string) bool {
	m, err := GetMethod(method)
	if err != nil {
		return false
	}
	for _, p := range m.Permission {
		if strings.ToLower(p) == "private" {
			return true
		}
	}
	return false
}

// HasAdminPermission checks if a method requires Admin access
func HasAdminPermission(method string) bool {
	m, err := GetMethod(method)
	if err != nil {
		return false
	}
	for _, p := range m.Permission {
		if p == "ADMIN" {
			return true
		}
	}
	return false
}

// GetMethodConfig finds a method config by name
func GetMethodConfig(methodName string) (*Method, error) {
	for _, method := range GetMethodsList() {
		if method.Method == methodName {
			return &method, nil
		}
	}
	return nil, errors.New("Method not found")
}

// SaveMethods writes the current cached methods back to the given file path.
func SaveMethods(path string) error {
	data, err := json.MarshalIndent(cachedMethods, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SetMethodEnabled enables or disables a method by name and persists to funnel.json.
func SetMethodEnabled(name string, enabled bool) error {
	if !methodsLoaded {
		LoadMethods()
	}
	for i, m := range cachedMethods {
		if m.Method == name {
			cachedMethods[i].Enabled = enabled
			return SaveMethods("assets/funnel/funnel.json")
		}
	}
	return errors.New("method not found: " + name)
}

// SetMethodSlots changes the slot limit for a method and persists to funnel.json.
func SetMethodSlots(name string, slots int) error {
	if !methodsLoaded {
		LoadMethods()
	}
	for i, m := range cachedMethods {
		if m.Method == name {
			cachedMethods[i].Slots = slots
			return SaveMethods("assets/funnel/funnel.json")
		}
	}
	return errors.New("method not found: " + name)
}
