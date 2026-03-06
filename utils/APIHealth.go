package utils

import (
	"sync"
	"time"
)

// APIStatus represents the last known health state of a single API endpoint
type APIStatus struct {
	URL       string
	Label     string
	LastOK    time.Time
	LastError time.Time
	OK        bool
}

var apiHealthMu sync.RWMutex
var apiHealth = map[string]*APIStatus{}

// RecordAPISuccess marks an endpoint as healthy
func RecordAPISuccess(url, label string) {
	apiHealthMu.Lock()
	defer apiHealthMu.Unlock()
	s, ok := apiHealth[url]
	if !ok {
		s = &APIStatus{URL: url, Label: label}
		apiHealth[url] = s
	}
	s.OK = true
	s.LastOK = time.Now()
}

// RecordAPIError marks an endpoint as unhealthy
func RecordAPIError(url, label string) {
	apiHealthMu.Lock()
	defer apiHealthMu.Unlock()
	s, ok := apiHealth[url]
	if !ok {
		s = &APIStatus{URL: url, Label: label}
		apiHealth[url] = s
	}
	s.OK = false
	s.LastError = time.Now()
}

// GetAllAPIStatuses returns a copy of all known endpoint statuses
func GetAllAPIStatuses() []APIStatus {
	apiHealthMu.RLock()
	defer apiHealthMu.RUnlock()
	out := make([]APIStatus, 0, len(apiHealth))
	for _, s := range apiHealth {
		out = append(out, *s)
	}
	return out
}
