package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mahdiXak47/Download-Manager/internal/logger"
)

type Monitor struct {
	checkInterval time.Duration
	stopChan      chan struct{}
	isConnected   bool
	lastCheck     time.Time
	mu            sync.RWMutex
	checkURL      string
}

// NewMonitor creates a new network monitor
func NewMonitor(checkInterval time.Duration, checkURL string) *Monitor {
	logger.LogDownloadEvent("NETWORK", fmt.Sprintf("Initializing network monitor with interval: %v, check URL: %s", checkInterval, checkURL))
	return &Monitor{
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		isConnected:   true, // Assume connected by default
		checkURL:      checkURL,
	}
}

// Start begins monitoring network connectivity
func (m *Monitor) Start() {
	logger.LogDownloadEvent("NETWORK", "Starting network monitor")
	go m.run()
}

// Stop halts the network monitoring
func (m *Monitor) Stop() {
	logger.LogDownloadEvent("NETWORK", "Stopping network monitor")
	close(m.stopChan)
}

// IsConnected returns the current network connectivity status
func (m *Monitor) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// run performs periodic network connectivity checks
func (m *Monitor) run() {
	logger.LogDownloadEvent("NETWORK", "Network monitor started running")
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			logger.LogDownloadEvent("NETWORK", "Network monitor stopped")
			return
		case <-ticker.C:
			m.checkConnection()
		}
	}
}

// checkConnection performs a network connectivity check
func (m *Monitor) checkConnection() {
	// Try to resolve the check URL first
	logger.LogDownloadEvent("NETWORK", fmt.Sprintf("Checking DNS resolution for %s", m.checkURL))
	_, err := net.LookupHost(m.checkURL)
	if err != nil {
		logger.LogDownloadEvent("NETWORK", fmt.Sprintf("DNS resolution failed: %v", err))
		m.updateStatus(false, "DNS resolution failed: "+err.Error())
		return
	}

	// Try to establish a TCP connection
	logger.LogDownloadEvent("NETWORK", fmt.Sprintf("Attempting TCP connection to %s:80", m.checkURL))
	conn, err := net.DialTimeout("tcp", m.checkURL+":80", 5*time.Second)
	if err != nil {
		logger.LogDownloadEvent("NETWORK", fmt.Sprintf("TCP connection failed: %v", err))
		m.updateStatus(false, "TCP connection failed: "+err.Error())
		return
	}
	conn.Close()

	logger.LogDownloadEvent("NETWORK", "Network connection check successful")
	m.updateStatus(true, "Network connection is active")
}

// updateStatus updates the connection status and logs changes
func (m *Monitor) updateStatus(connected bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Only log if status has changed
	if m.isConnected != connected {
		m.isConnected = connected
		m.lastCheck = time.Now()

		if connected {
			logger.LogDownloadEvent("NETWORK", "Network connection restored: "+message)
		} else {
			logger.LogDownloadEvent("NETWORK", "Network connection lost: "+message)
		}
	}
}
