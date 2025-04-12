package tui

import (
	"fmt"
	// "net/http"
	// "strings"
	"path/filepath"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mahdiXak47/Download-Manager/internal/config"
	"github.com/mahdiXak47/Download-Manager/internal/downloader"
	"github.com/mahdiXak47/Download-Manager/internal/queue"
)

// TabID represents different tabs in the application
type TabID int

const (
	AddDownloadTab TabID = iota
	DownloadListTab
	QueueListTab
	SettingsTab
)

// Model represents the application state
type Model struct {
	// Core state
	ActiveTab     TabID  // Current active tab
	Menu          string // "main", "add", "list"
	InputMode     bool   // Whether we're capturing input
	Selected      int    // Currently selected download
	QueueSelected int    // Currently selected queue

	// Add Download state
	QueueSelectionMode bool   // Whether we're in queue selection mode
	URLInputMode       bool   // Whether we're in URL input mode
	AddDownloadMessage string // Message shown after an add download operation
	AddDownloadSuccess bool   // Whether the last add was successful (for coloring)

	// Download List state
	DownloadListMessage string // Message shown in the download list tab
	DownloadListSuccess bool   // Whether the last download list operation was successful (for coloring)

	// Input fields
	InputURL   string
	InputQueue string

	// Input fields for queue form
	InputQueueName       string
	InputQueuePath       string
	InputQueueConcurrent string
	InputQueueSpeedLimit string
	InputQueueStartTime  string
	InputQueueEndTime    string
	QueueFormMode        bool // Whether we're in queue form mode
	QueueFormField       int  // Current field in queue form

	// Add input fields for scheduled start date and time
	InputScheduledStartDate string // New field for scheduled start date
	InputScheduledStartTime string // New field for scheduled start time

	// Data
	Downloads    []downloader.Download
	Config       *config.Config
	QueueManager *queue.Manager
	ErrorMessage string

	// UI State
	Width  int
	Height int

	CurrentTheme string

	// Add fields to track download start and completion times
	StartTime      time.Time
	CompletionTime time.Time

	PopupMessage string
	PopupVisible bool
	PopupType    string // "error", "success", "info"
}

// NewModel creates and initializes a new model
func NewModel() Model {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return Model{
			ActiveTab:    DownloadListTab,
			Menu:         "list",
			Downloads:    make([]downloader.Download, 0),
			Selected:     0,
			Width:        80,
			Height:       24,
			ErrorMessage: "Failed to load config: " + err.Error(),
		}
	}

	// Create queue manager
	queueManager := queue.NewManager(cfg)
	queueManager.Start()

	return Model{
		ActiveTab:          DownloadListTab,
		Menu:               "list",
		Downloads:          cfg.Downloads,
		Config:             cfg,
		QueueManager:       queueManager,
		Selected:           0,
		QueueSelected:      0,
		QueueSelectionMode: false,
		URLInputMode:       false,
		AddDownloadSuccess: false,
		Width:              80,
		Height:             24,
		CurrentTheme:       "modern", // Default theme
	}
}

// Init runs any initial IO
func (m Model) Init() tea.Cmd {
	return nil
}

// HandleInput processes text input when in input mode
func (m *Model) HandleInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel input and clear fields
		if m.QueueFormMode {
			m.QueueFormMode = false
			m.InputQueueName = ""
			m.InputQueuePath = ""
			m.InputQueueConcurrent = ""
			m.InputQueueSpeedLimit = ""
			m.InputQueueStartTime = ""
			m.InputQueueEndTime = ""
			m.QueueFormField = 0
		} else {
			m.InputMode = false
			m.InputURL = ""
			m.InputQueue = ""
		}
		return *m, nil

	case tea.KeyBackspace:
		if m.QueueFormMode {
			switch m.QueueFormField {
			case 0:
				if len(m.InputQueueName) > 0 {
					m.InputQueueName = m.InputQueueName[:len(m.InputQueueName)-1]
				}
			case 1:
				// Restrict editing of the file path in queue form mode
				// m.InputQueuePath += string(msg.Runes)
			case 2:
				if len(m.InputQueueConcurrent) > 0 {
					m.InputQueueConcurrent = m.InputQueueConcurrent[:len(m.InputQueueConcurrent)-1]
				}
			case 3:
				if len(m.InputQueueSpeedLimit) > 0 {
					m.InputQueueSpeedLimit = m.InputQueueSpeedLimit[:len(m.InputQueueSpeedLimit)-1]
				}
			case 4:
				if len(m.InputQueueStartTime) > 0 {
					m.InputQueueStartTime = m.InputQueueStartTime[:len(m.InputQueueStartTime)-1]
				}
			case 5:
				if len(m.InputQueueEndTime) > 0 {
					m.InputQueueEndTime = m.InputQueueEndTime[:len(m.InputQueueEndTime)-1]
				}
			}
		} else if m.InputMode {
			if len(m.InputURL) > 0 {
				m.InputURL = m.InputURL[:len(m.InputURL)-1]
			}
		}
		return *m, nil

	case tea.KeyRunes:
		if m.QueueFormMode {
			switch m.QueueFormField {
			case 0:
				m.InputQueueName += string(msg.Runes)
			case 1:
				// Restrict editing of the file path in queue form mode
				// m.InputQueuePath += string(msg.Runes)
			case 2:
				m.InputQueueConcurrent += string(msg.Runes)
			case 3:
				m.InputQueueSpeedLimit += string(msg.Runes)
			case 4:
				m.InputQueueStartTime += string(msg.Runes)
			case 5:
				m.InputQueueEndTime += string(msg.Runes)
			}
		} else if m.InputMode {
			m.InputURL += string(msg.Runes)
		}
		return *m, nil
	}

	return *m, nil
}

// UpdateSize updates the model's terminal size
func (m *Model) UpdateSize(width, height int) {
	m.Width = width
	m.Height = height
}

// AddDownload adds a new download to the model
func (m *Model) AddDownload(url, queue string) {
	if queue == "" {
		queue = m.Config.DefaultQueue
	}

	// Get the queue configuration to set bandwidth limit
	var maxBandwidth int64 = 0
	for _, q := range m.Config.Queues {
		if q.Name == queue {
			maxBandwidth = q.SpeedLimit
			break
		}
	}

	// Get a proper target path from the URL
	filename := filepath.Base(url)
	queuePath := m.Config.SavePath // Default to the global SavePath

	// Find the queue configuration
	for _, q := range m.Config.Queues {
		if q.Name == queue {
			// If the queue has a configured Path, use it
			if q.Path != "" {
				queuePath = q.Path
			}
			break
		}
	}

	var targetPath string
	if queuePath != "" {
		targetPath = filepath.Join(queuePath, filename)
	} else {
		// Use current directory if no path specified
		targetPath = filename
	}

	// Create and initialize download object
	scheduledStartTime := time.Now() // Default to now
	if m.InputScheduledStartDate != "" && m.InputScheduledStartTime != "" {
		scheduledStartTime, _ = time.Parse("2006-01-02 15:04", m.InputScheduledStartDate+" "+m.InputScheduledStartTime)
	}
	download := downloader.New(url, targetPath, queue, maxBandwidth, scheduledStartTime)
	m.Downloads = append(m.Downloads, *download)

	// Add to queue manager's downloads map for tracking
	m.QueueManager.ProcessDownload(url)

	// Process all queues immediately to start the download
	m.QueueManager.ProcessAllQueues()

	// Set the start time
	download.StartTime = time.Now()

	// Update config with new download
	if m.Config != nil {
		m.Config.Downloads = m.Downloads
		if err := config.SaveConfig(m.Config); err != nil {
			m.ErrorMessage = "Failed to save config: " + err.Error()
		}
	}
}

// PauseDownload pauses the selected download
func (m *Model) PauseDownload() {
	if m.Selected >= 0 && m.Selected < len(m.Downloads) {
		download := &m.Downloads[m.Selected]
		if download.Status == "downloading" {
			// Set completion time to zero if paused
			download.CompletionTime = time.Time{}
			m.QueueManager.PauseDownload(download.URL)
			m.ShowPopup(fmt.Sprintf("Paused download: %s", download.Filename), "info")
		} else {
			m.ShowPopup("Can only pause downloads that are in progress", "error")
		}
	}
}

// ResumeDownload resumes the selected download
func (m *Model) ResumeDownload() {
	if m.Selected >= 0 && m.Selected < len(m.Downloads) {
		download := &m.Downloads[m.Selected]
		if download.Status == "paused" {
			// Reset start time when resuming
			download.StartTime = time.Now()
			m.QueueManager.ResumeDownload(download.URL)
			m.ShowPopup(fmt.Sprintf("Resumed download: %s", download.Filename), "info")
		} else {
			m.ShowPopup("Can only resume downloads that are paused", "error")
		}
	}
}

// CancelDownload removes the selected download from the queue and downloads list
func (m *Model) CancelDownload() {
	if m.Selected >= 0 && m.Selected < len(m.Downloads) {
		download := m.Downloads[m.Selected]

		// Set completion time if download is active
		if download.Status == "downloading" || download.Status == "paused" {
			download.CompletionTime = time.Now()
			// Cancel the download if it's active
			if download.Status == "downloading" || download.Status == "paused" {
				download.Cancel()
			}

			// Remove from queue manager
			m.QueueManager.RemoveDownload(download.URL)

			// Remove from downloads list
			m.Downloads = append(m.Downloads[:m.Selected], m.Downloads[m.Selected+1:]...)

			// Adjust selection if needed
			if m.Selected >= len(m.Downloads) {
				m.Selected = len(m.Downloads) - 1
			}

			// Update config
			if m.Config != nil {
				m.Config.Downloads = m.Downloads
				if err := config.SaveConfig(m.Config); err != nil {
					m.ErrorMessage = "Failed to save config: " + err.Error()
				}
			}
		}
	}
}

// CycleTheme switches to the next available theme
func (m *Model) CycleTheme() {
	themes := map[string]Theme{
		"modern":    ModernTheme,
		"ocean":     OceanTheme,
		"solarized": SolarizedTheme,
		"nord":      NordTheme,
		"synthwave": SynthwaveTheme,
		"dracula":   DraculaTheme,
		"cyberpunk": CyberpunkTheme,
		"retro":     RetroTheme,
		"neon":      NeonTheme,
		"aurora":    AuroraTheme,
	}

	// Get ordered theme names
	themeNames := []string{
		"modern",
		"ocean",
		"solarized",
		"nord",
		"synthwave",
		"dracula",
		"cyberpunk",
		"retro",
		"neon",
		"aurora",
	}

	currentIndex := 0
	for i, name := range themeNames {
		if name == m.CurrentTheme {
			currentIndex = i
			break
		}
	}

	// Switch to next theme
	nextIndex := (currentIndex + 1) % len(themeNames)
	m.CurrentTheme = themeNames[nextIndex]
	CurrentTheme = themes[m.CurrentTheme]
	UpdateStyles()
}

// SaveQueueForm saves the current queue form values to the config
func (m *Model) SaveQueueForm() error {
	// Convert string inputs to appropriate types
	maxConcurrent := 3 // Default
	if m.InputQueueConcurrent != "" {
		if val, err := strconv.Atoi(m.InputQueueConcurrent); err == nil && val > 0 {
			maxConcurrent = val
		}
	}

	var speedLimit int64 = 0 // Default - unlimited
	if m.InputQueueSpeedLimit != "" {
		if val, err := strconv.ParseInt(m.InputQueueSpeedLimit, 10, 64); err == nil && val >= 0 {
			speedLimit = val
		}
	}

	// Validate time formats
	startTime := "00:00" // Default
	if m.InputQueueStartTime != "" {
		if _, err := time.Parse("15:04", m.InputQueueStartTime); err == nil {
			startTime = m.InputQueueStartTime
		}
	}

	endTime := "23:59" // Default
	if m.InputQueueEndTime != "" {
		if _, err := time.Parse("15:04", m.InputQueueEndTime); err == nil {
			endTime = m.InputQueueEndTime
		}
	}

	// Create the queue config
	queue := config.QueueConfig{
		Name:          m.InputQueueName,
		Path:          m.InputQueuePath,
		MaxConcurrent: maxConcurrent,
		SpeedLimit:    speedLimit,
		StartTime:     startTime,
		EndTime:       endTime,
		Enabled:       true,
	}

	// Check if we're editing an existing queue or creating a new one
	found := false
	for i, q := range m.Config.Queues {
		if q.Name == m.InputQueueName {
			// Update existing queue
			m.Config.Queues[i] = queue
			found = true
			break
		}
	}

	if !found {
		// Add new queue
		m.Config.Queues = append(m.Config.Queues, queue)
	}

	// Save config
	return config.SaveConfig(m.Config)
}

// ShowPopup shows a popup message
func (m *Model) ShowPopup(message string, popupType string) {
	m.PopupMessage = message
	m.PopupVisible = true
	m.PopupType = popupType
}

// HidePopup hides the popup message
func (m *Model) HidePopup() {
	m.PopupVisible = false
	m.PopupMessage = ""
	m.PopupType = ""
}

// RetryDownload retries the selected download if it's in error state
func (m *Model) RetryDownload() {
	if m.Selected >= 0 && m.Selected < len(m.Downloads) {
		download := &m.Downloads[m.Selected]

		// Check if download is in error state
		if download.Status == "error" {
			// Check if retry count is less than max retries (3)
			if download.GetRetryCount() < 3 {
				// Retry the download
				err := download.Retry()
				if err != nil {
					m.ShowPopup(fmt.Sprintf("Error: %s", err.Error()), "error")
				} else {
					m.ShowPopup(fmt.Sprintf("Trying again to download file #%d", m.Selected+1), "success")

					// Queue the download for processing
					m.QueueManager.ProcessDownload(download.URL)

					// Update config
					if m.Config != nil {
						if err := config.SaveConfig(m.Config); err != nil {
							m.ShowPopup(fmt.Sprintf("Failed to save config: %s", err.Error()), "error")
						}
					}
				}
			} else {
				// Max retries reached
				m.ShowPopup("Error: Maximum retry attempts (3) reached for this download", "error")
			}
		} else {
			// Not in error state
			m.ShowPopup("Error: Only downloads in error state can be retried", "error")
		}
	}
}
