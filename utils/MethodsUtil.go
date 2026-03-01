package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Method represents an attack method configuration from funnel.json
type Method struct {
	Enabled           bool     `json:"enabled"`
	EnabledWithFunnel bool     `json:"enabledWithFunnel"`
	Method            string   `json:"method"`
	DefaultPort       uint16   `json:"defaultPort"`
	DefaultTime       uint32   `json:"defaultTime"`
	MinTime           uint32   `json:"minTime"`
	MaxTime           uint32   `json:"maxTime"`
	Permission        []string `json:"permission"`
	Slots             int      `json:"slots"`
	API               []string `json:"api"`
}

var cachedMethods []Method
var methodsLoaded bool

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
