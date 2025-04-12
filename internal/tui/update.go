package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mahdiXak47/Download-Manager/internal/config"
)

// Update handles all state updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return handleWindowSize(m, msg)
	case tea.KeyMsg:
		return handleKeyPress(m, msg)
	case TickMsg:
		return handleTick(m)
	case StartDownloadMsg:
		return handleStartDownload(m, msg)
	case DownloadProgressMsg:
		return handleProgress(m, msg)
	case ErrorMsg:
		return handleError(m, msg)
	}
	return m, nil
}

// handleWindowSize updates the terminal size
func handleWindowSize(m Model, msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.UpdateSize(msg.Width, msg.Height)
	return m, nil
}

// handleKeyPress handles all keyboard input
func handleKeyPress(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When in URL input mode, only handle Esc and Enter keys, pass everything else to text input handler
	if m.URLInputMode {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel URL input and go back
			m.URLInputMode = false
			m.InputURL = ""
			return m, nil
		case tea.KeyEnter:
			// Validate and start download
			if m.InputURL != "" {
				// Check if the URL is valid
				isValidURL := validateURL(m.InputURL)
				if !isValidURL {
					m.AddDownloadMessage = "Error: Invalid URL format"
					m.AddDownloadSuccess = false
					m.URLInputMode = false
					return m, nil
				}

				// Check if the queue has capacity
				queueName := m.InputQueue
				var queue *config.QueueConfig
				for _, q := range m.Config.Queues {
					if q.Name == queueName {
						queue = &q
						break
					}
				}

				if queue != nil {
					// Count active downloads in this queue
					activeCount := 0
					for _, d := range m.Downloads {
						if d.Queue == queueName && d.Status == "downloading" {
							activeCount++
						}
					}

					if activeCount >= queue.MaxConcurrent {
						m.AddDownloadMessage = fmt.Sprintf("Error: Queue '%s' is at maximum capacity (%d downloads)", queueName, queue.MaxConcurrent)
						m.AddDownloadSuccess = false
						m.URLInputMode = false
						return m, nil
					}

					// Store URL before clearing it
					url := m.InputURL

					// All checks passed, start the download
					cmd := func() tea.Msg {
						return StartDownloadMsg{
							URL:   url,
							Queue: m.InputQueue,
						}
					}

					m.AddDownloadMessage = fmt.Sprintf("Success: Download started in queue '%s'", queueName)
					m.AddDownloadSuccess = true
					m.URLInputMode = false
					m.InputURL = ""

					return m, cmd
				} else {
					m.AddDownloadMessage = "Error: Selected queue not found"
					m.AddDownloadSuccess = false
					m.URLInputMode = false
					return m, nil
				}
			}
			return m, nil
		case tea.KeyBackspace:
			// Handle backspace
			if len(m.InputURL) > 0 {
				m.InputURL = m.InputURL[:len(m.InputURL)-1]
			}
			return m, nil
		default:
			// Handle all other keys as text input
			if msg.Type == tea.KeyRunes {
				m.InputURL += string(msg.Runes)
			}
			return m, nil
		}
	}

	// When in Queue selection mode, handle navigation separately
	if m.QueueSelectionMode {
		switch msg.String() {
		case "up", "k":
			if m.QueueSelected > 0 {
				m.QueueSelected--
			} else if len(m.Config.Queues) > 0 {
				m.QueueSelected = len(m.Config.Queues) - 1
			}
		case "down", "j":
			if m.QueueSelected < len(m.Config.Queues)-1 {
				m.QueueSelected++
			} else {
				m.QueueSelected = 0
			}
		case "enter":
			if len(m.Config.Queues) > 0 {
				// Select the queue and move to URL input
				m.InputQueue = m.Config.Queues[m.QueueSelected].Name
				m.QueueSelectionMode = false
				m.URLInputMode = true
				m.InputURL = ""
			}
		case "esc":
			// Cancel queue selection
			m.QueueSelectionMode = false
		}
		return m, nil
	}

	// When in Queue form mode, handle form input
	if m.QueueFormMode {
		return handleQueueFormInput(m, msg)
	}

	// Handle global keys first (when not in any input mode)
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		// ESC in base mode clears messages and resets state
		if m.PopupVisible {
			m.HidePopup()
			return m, nil
		}
		if m.ActiveTab == AddDownloadTab && m.AddDownloadMessage != "" {
			// Clear any download messages
			m.AddDownloadMessage = ""
			m.AddDownloadSuccess = false
			return m, nil
		}
		return m, nil
	}

	// Handle number keys for tab switching (when not in input mode)
	switch msg.String() {
	case "1":
		m.ActiveTab = AddDownloadTab
		m.Menu = "add"
		// Reset add download state
		if m.AddDownloadMessage != "" {
			m.AddDownloadMessage = ""
			m.AddDownloadSuccess = false
		}
		return m, nil
	case "2":
		m.ActiveTab = DownloadListTab
		m.Menu = "list"
		return m, nil
	case "3":
		m.ActiveTab = QueueListTab
		m.Menu = "queues"
		return m, nil
	case "4":
		m.ActiveTab = SettingsTab
		m.Menu = "settings"
		return m, nil
	case "q":
		return m, tea.Quit
	case "t":
		m.CycleTheme()
		return m, nil
	}

	// Handle tab-specific keys
	switch m.ActiveTab {
	case AddDownloadTab:
		return handleAddDownloadTab(m, msg)
	case DownloadListTab:
		return handleDownloadListTab(m, msg)
	case QueueListTab:
		return handleQueueListTab(m, msg)
	case SettingsTab:
		return handleSettingsTab(m, msg)
	}

	return m, nil
}

// handleInputMode handles keyboard input when in input mode
func handleInputMode(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.Menu == "add" && m.InputURL != "" {
			cmd := func() tea.Msg {
				return StartDownloadMsg{
					URL:   m.InputURL,
					Queue: m.InputQueue,
				}
			}
			m.InputMode = false
			m.InputURL = ""
			m.InputQueue = ""
			m.Menu = "list"
			return m, cmd
		}

	case tea.KeyBackspace:
		if len(m.InputURL) > 0 {
			m.InputURL = m.InputURL[:len(m.InputURL)-1]
		}

	case tea.KeyRunes:
		m.InputURL += string(msg.Runes)
	}

	return m, nil
}

// Handles navigation (e.g., switching menus, pausing/resuming downloads).
func handleNavigationMode(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.Menu == "list" && len(m.Downloads) > 0 {
			if m.Selected > 0 {
				m.Selected--
			} else {
				m.Selected = len(m.Downloads) - 1
			}
		}
	case tea.KeyDown:
		if m.Menu == "list" && len(m.Downloads) > 0 {
			m.Selected = (m.Selected + 1) % len(m.Downloads)
		}
	}

	switch msg.String() {
	case "a":
		m.Menu = "add"
		m.InputMode = true
	case "l":
		m.Menu = "list"
	case "p":
		if m.Menu == "list" {
			m.PauseDownload()
		}
	case "s":
		if m.Menu == "list" {
			m.ResumeDownload()
			return m, tickCmd()
		}
	case "c":
		if m.Menu == "list" {
			m.CancelDownload()
		}
	case "j":
		if m.Menu == "list" && len(m.Downloads) > 0 {
			m.Selected = (m.Selected + 1) % len(m.Downloads)
		}
	case "k":
		if m.Menu == "list" && len(m.Downloads) > 0 {
			if m.Selected > 0 {
				m.Selected--
			} else {
				m.Selected = len(m.Downloads) - 1
			}
		}
	}

	return m, nil
}

// handleStartDownload processes a new download request
func handleStartDownload(m Model, msg StartDownloadMsg) (tea.Model, tea.Cmd) {
	m.AddDownload(msg.URL, msg.Queue)

	// Custom command to help with UI refresh after adding a download
	var cmd tea.Cmd = func() tea.Msg {
		// Wait briefly for download to start
		time.Sleep(300 * time.Millisecond)
		return TickMsg{}
	}

	return m, cmd
}

// handleProgress updates download progress
func handleProgress(m Model, msg DownloadProgressMsg) (tea.Model, tea.Cmd) {
	for i, d := range m.Downloads {
		if d.URL == msg.URL {
			m.Downloads[i].Progress = msg.Progress
			m.Downloads[i].Speed = msg.Speed
			break
		}
	}
	return m, nil
}

// handleError displays error messages
func handleError(m Model, msg ErrorMsg) (tea.Model, tea.Cmd) {
	m.ErrorMessage = msg.Error.Error()
	return m, nil
}

// Handles periodic updates (e.g., checking for active downloads).
func handleTick(m Model) (tea.Model, tea.Cmd) {
	// Update active downloads
	hasActive := false
	for _, d := range m.Downloads {
		if d.Status == "downloading" || d.Status == "paused" {
			hasActive = true
			break
		}
	}

	if hasActive {
		return m, tickCmd()
	}
	return m, nil
}

// Schedules a periodic update
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/2, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// handleAddDownloadTab handles keys for the Add Download tab
func handleAddDownloadTab(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// We only need to handle the initial "enter" press to start the process
	// The queue selection and URL modes are handled in handleKeyPress
	if !m.QueueSelectionMode && !m.URLInputMode {
		switch msg.String() {
		case "enter":
			// Start the process by showing queue selection
			if len(m.Config.Queues) > 0 {
				m.QueueSelectionMode = true
				m.QueueSelected = 0 // Select first queue by default

				// Clear any previous messages
				m.AddDownloadMessage = ""
				m.AddDownloadSuccess = false
			} else {
				m.AddDownloadMessage = "Error: No queues configured. Please create a queue first."
				m.AddDownloadSuccess = false
			}
		case "esc":
			// Also clear any message when ESC is pressed in this context
			m.AddDownloadMessage = ""
			m.AddDownloadSuccess = false
		}
	}

	return m, nil
}

// handleDownloadListTab handles keys for the Download List tab
func handleDownloadListTab(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.Selected > 0 {
			m.Selected--
		}
	case "down", "j":
		if m.Selected < len(m.Downloads)-1 {
			m.Selected++
		}
	case "p":
		m.PauseDownload()
	case "r":
		m.ResumeDownload()
		return m, tickCmd()
	case "c":
		m.CancelDownload()
	case "y":
		// Retry the selected download if it's in error state
		m.RetryDownload()
	case "a":
		// Switch to Add Download tab
		m.ActiveTab = AddDownloadTab
		m.Menu = "add"
	case "d":
		// Delete the selected download
		if m.Selected >= 0 && m.Selected < len(m.Downloads) {
			selectedDownload := m.Downloads[m.Selected]
			m.QueueManager.RemoveDownload(selectedDownload.URL)
			m.Downloads = append(m.Downloads[:m.Selected], m.Downloads[m.Selected+1:]...)
			if m.Selected >= len(m.Downloads) {
				m.Selected = len(m.Downloads) - 1
			}
		}
	case "esc":
		// Clear any messages
		if m.DownloadListMessage != "" {
			m.DownloadListMessage = ""
			m.DownloadListSuccess = false
		}
	}

	return m, nil
}

// handleQueueListTab handles keys for the Queue List tab
func handleQueueListTab(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.QueueFormMode {
		// Handle queue form input
		switch msg.String() {
		case "up", "shift+tab":
			if m.QueueFormField > 0 {
				m.QueueFormField--
			}
		case "down", "tab":
			if m.QueueFormField < 5 { // 6 fields total (0-5)
				m.QueueFormField++
			}
		case "enter":
			if m.QueueFormField < 5 {
				// Move to next field
				m.QueueFormField++
			} else {
				// Submit form
				m.SaveQueueForm()
				m.QueueFormMode = false
			}
		default:
			// Handle text input
			m, cmd := m.HandleInput(msg)
			return m, cmd
		}
		return m, nil
	}

	// Not in form mode
	switch msg.String() {
	case "up", "k":
		if m.QueueSelected > 0 {
			m.QueueSelected--
		}
	case "down", "j":
		if m.QueueSelected < len(m.Config.Queues)-1 {
			m.QueueSelected++
		}
	case "n":
		// New queue
		m.QueueFormMode = true
		m.QueueFormField = 0
		m.InputQueueName = ""
		m.InputQueuePath = ""
		m.InputQueueConcurrent = "3"
		m.InputQueueSpeedLimit = "0"
		m.InputQueueStartTime = "00:00"
		m.InputQueueEndTime = "23:59"
	case "e":
		// Edit queue
		if m.QueueSelected >= 0 && m.QueueSelected < len(m.Config.Queues) {
			q := m.Config.Queues[m.QueueSelected]
			m.QueueFormMode = true
			m.QueueFormField = 0
			m.InputQueueName = q.Name
			m.InputQueuePath = q.Path
			m.InputQueueConcurrent = fmt.Sprintf("%d", q.MaxConcurrent)
			m.InputQueueSpeedLimit = fmt.Sprintf("%d", q.SpeedLimit)
			m.InputQueueStartTime = q.StartTime
			m.InputQueueEndTime = q.EndTime
		}
	case "d":
		// Delete queue
		if m.QueueSelected >= 0 && m.QueueSelected < len(m.Config.Queues) {
			// Don't delete default queue
			if m.Config.Queues[m.QueueSelected].Name != m.Config.DefaultQueue {
				m.Config.Queues = append(m.Config.Queues[:m.QueueSelected], m.Config.Queues[m.QueueSelected+1:]...)
				if m.QueueSelected >= len(m.Config.Queues) {
					m.QueueSelected = len(m.Config.Queues) - 1
				}
				config.SaveConfig(m.Config)
			}
		}
	}

	return m, nil
}

// handleSettingsTab handles keys for the Settings tab
func handleSettingsTab(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// No need to handle 't' here as it's handled globally
	return m, nil
}

// handleQueueFormInput handles keyboard input for the queue form
func handleQueueFormInput(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "shift+tab":
		if m.QueueFormField > 0 {
			m.QueueFormField--
		}
	case "down", "tab":
		if m.QueueFormField < 5 { // 6 fields total (0-5)
			m.QueueFormField++
		}
	case "enter":
		if m.QueueFormField < 5 {
			// Move to next field
			m.QueueFormField++
		} else {
			// Submit form
			if err := m.SaveQueueForm(); err != nil {
				m.ErrorMessage = fmt.Sprintf("Error saving queue: %v", err)
			} else {
				m.ErrorMessage = ""
			}
			m.QueueFormMode = false
		}
	case "esc":
		// Cancel form
		m.QueueFormMode = false
		m.InputQueueName = ""
		m.InputQueuePath = ""
		m.InputQueueConcurrent = ""
		m.InputQueueSpeedLimit = ""
		m.InputQueueStartTime = ""
		m.InputQueueEndTime = ""
		m.QueueFormField = 0
	default:
		// Handle text input based on current field
		if msg.Type == tea.KeyBackspace {
			switch m.QueueFormField {
			case 0:
				if len(m.InputQueueName) > 0 {
					m.InputQueueName = m.InputQueueName[:len(m.InputQueueName)-1]
				}
			case 1:
				if len(m.InputQueuePath) > 0 {
					m.InputQueuePath = m.InputQueuePath[:len(m.InputQueuePath)-1]
				}
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
		} else if msg.Type == tea.KeyRunes {
			switch m.QueueFormField {
			case 0:
				m.InputQueueName += string(msg.Runes)
			case 1:
				m.InputQueuePath += string(msg.Runes)
			case 2:
				m.InputQueueConcurrent += string(msg.Runes)
			case 3:
				m.InputQueueSpeedLimit += string(msg.Runes)
			case 4:
				m.InputQueueStartTime += string(msg.Runes)
			case 5:
				m.InputQueueEndTime += string(msg.Runes)
			}
		}
	}

	return m, nil
}

// validateURL checks if the provided string is a valid URL
func validateURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return false
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		fmt.Println("Invalid URL: Scheme must be http or https")
		return false
	}

	// Check host
	if parsedURL.Host == "" {
		fmt.Println("Invalid URL: Missing host")
		return false
	}

	// Optionally, check if host contains at least one dot
	if !strings.Contains(parsedURL.Host, ".") {
		fmt.Println("Invalid URL: Host does not look valid")
		return false
	}

	fmt.Println("URL is valid")
	return true
}
