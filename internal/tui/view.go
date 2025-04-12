package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mahdiXak47/Download-Manager/internal/tui/styles"
)

func (m Model) View() string {
	// Create main container with dynamic width
	mainContainer := containerStyle.Width(m.Width - 4)

	// Header with app name and version
	header := titleStyle.Width(m.Width - 8).Render("Download Manager v0.1")

	// Build the content
	var content strings.Builder
	content.WriteString(header)

	// Error message if any
	if m.ErrorMessage != "" {
		content.WriteString("\n" + errorStyle.Render(m.ErrorMessage))
	}

	// Content based on the active tab
	var tabContent string
	switch m.ActiveTab {
	case AddDownloadTab:
		tabContent = renderAddDownloadTab(m)
	case DownloadListTab:
		tabContent = renderDownloadListTab(m)
	case QueueListTab:
		tabContent = renderQueueListTab(m)
	case SettingsTab:
		tabContent = renderSettingsTab(m)
	}
	content.WriteString("\n" + tabContent)

	// Tab bar at the bottom
	tabBar := renderTabBar(m)
	content.WriteString("\n" + tabBar)

	// Add popup message if visible
	if m.PopupVisible {
		var popupStyle lipgloss.Style
		switch m.PopupType {
		case "error":
			popupStyle = styles.ErrorStyle
		case "success":
			popupStyle = styles.SuccessStyle
		case "info":
			popupStyle = styles.InfoStyle
		default:
			popupStyle = styles.InfoStyle
		}

		// Add help text to the popup message
		popupContent := fmt.Sprintf("%s\n\nPress ESC to close", m.PopupMessage)
		popup := popupStyle.
			Width(50).
			Align(lipgloss.Center).
			Padding(1, 2).
			Render(popupContent)

		// Center the popup
		popup = lipgloss.Place(
			m.Width, m.Height,
			lipgloss.Center, lipgloss.Center,
			popup,
		)

		// Overlay the popup on top of the main content
		content.WriteString("\n" + popup)
	}

	// Wrap everything in the main container
	return mainContainer.Render(content.String())
}

// renderTabBar creates the tab bar with number keys 1-4
func renderTabBar(m Model) string {
	width := m.Width - 12 // Account for margins/paddings
	tabWidth := width / 4

	// Create tab styles based on active tab
	tab1Style := tabStyle.Copy().Width(tabWidth)
	tab2Style := tabStyle.Copy().Width(tabWidth)
	tab3Style := tabStyle.Copy().Width(tabWidth)
	tab4Style := tabStyle.Copy().Width(tabWidth)

	// Highlight active tab with more distinctive styling
	switch m.ActiveTab {
	case AddDownloadTab:
		tab1Style = activeTabStyle.Copy().Width(tabWidth)
	case DownloadListTab:
		tab2Style = activeTabStyle.Copy().Width(tabWidth)
	case QueueListTab:
		tab3Style = activeTabStyle.Copy().Width(tabWidth)
	case SettingsTab:
		tab4Style = activeTabStyle.Copy().Width(tabWidth)
	}

	// Render tabs with number key indicators
	tab1 := tab1Style.Render("1: Add Download")
	tab2 := tab2Style.Render("2: Download List")
	tab3 := tab3Style.Render("3: Queues List")
	tab4 := tab4Style.Render("4: Settings")

	return lipgloss.JoinHorizontal(lipgloss.Top, tab1, tab2, tab3, tab4)
}

func renderAddDownloadTab(m Model) string {
	var s strings.Builder

	// Center all content
	centerContainer := centerStyle.Copy().Width(m.Width - 8)

	s.WriteString(centerContainer.Render(menuHeaderStyle.Render("Add New Download")))
	s.WriteString("\n\n")

	// Success or error message (if any)
	if m.AddDownloadMessage != "" {
		msgStyle := errorStyle
		if m.AddDownloadSuccess {
			msgStyle = msgStyle.Copy().
				Foreground(lipgloss.Color(CurrentTheme.Special.Dark)).
				BorderForeground(lipgloss.Color(CurrentTheme.Special.Dark))
		} else {
			msgStyle = msgStyle.Copy().
				Foreground(lipgloss.Color(CurrentTheme.Error.Dark)).
				BorderForeground(lipgloss.Color(CurrentTheme.Error.Dark))
		}
		s.WriteString(centerContainer.Render(msgStyle.Render(m.AddDownloadMessage)) + "\n\n")
	}

	// Queue selection first
	if m.QueueSelectionMode {
		s.WriteString(centerContainer.Render(menuHeaderStyle.Render("Select Download Queue")))
		s.WriteString("\n\n")

		// Available queues
		s.WriteString(centerContainer.Render(menuItemStyle.Render("Available Queues:")))
		s.WriteString("\n\n")

		// List all queues
		for i, q := range m.Config.Queues {
			itemStyle := menuItemStyle
			if i == m.QueueSelected {
				itemStyle = selectedItemStyle
			}

			activeCount := 0
			for _, d := range m.Downloads {
				if d.Queue == q.Name && d.Status == "downloading" {
					activeCount++
				}
			}

			queueInfo := fmt.Sprintf("%s (%d/%d active)", q.Name, activeCount, q.MaxConcurrent)
			s.WriteString(centerContainer.Render(itemStyle.Render(queueInfo)) + "\n")
		}

		// Help text
		s.WriteString("\n" + helpStyle.Width(m.Width).Render("[ ↑/↓ ] Navigate   [ Enter ] Select   [ Esc ] Cancel"))
	} else if m.URLInputMode {
		s.WriteString(centerContainer.Render(menuHeaderStyle.Render("Enter Download URL")))
		s.WriteString("\n\n")

		// Selected queue display
		s.WriteString(centerContainer.Render(menuItemStyle.Render("Selected Queue: " + urlStyle.Render(m.InputQueue))))
		s.WriteString("\n\n")

		// URL input field
		s.WriteString(centerContainer.Render(inputBoxStyle.Render(
			menuItemStyle.Render("URL: " + urlStyle.Render(m.InputURL+"_")),
		)))

		// Help text for input mode
		s.WriteString("\n\n" + helpStyle.Width(m.Width).Render("[ Enter ] Start Download   [ Esc ] Back"))
	} else {
		// Initial instructions
		s.WriteString(centerContainer.Render(menuItemStyle.Render("Press Enter to add a new download")))
		s.WriteString("\n\n" + helpStyle.Width(m.Width).Render("[ Enter ] Start   [ Esc ] Back"))
	}

	return s.String()
}

func renderDownloadListTab(m Model) string {
	var s strings.Builder

	// Center all content
	centerContainer := centerStyle.Copy().Width(m.Width - 8)

	s.WriteString(centerContainer.Render(menuHeaderStyle.Render("Download List")))
	s.WriteString("\n\n")

	if m.DownloadListMessage != "" {
		msgStyle := errorStyle
		if m.DownloadListSuccess {
			msgStyle = msgStyle.Copy().
				Foreground(lipgloss.Color(CurrentTheme.Special.Dark)).
				BorderForeground(lipgloss.Color(CurrentTheme.Special.Dark))
		} else {
			msgStyle = msgStyle.Copy().
				Foreground(lipgloss.Color(CurrentTheme.Error.Dark)).
				BorderForeground(lipgloss.Color(CurrentTheme.Error.Dark))
		}
		s.WriteString(centerContainer.Render(msgStyle.Render(m.DownloadListMessage)) + "\n\n")
	}

	if len(m.Downloads) == 0 {
		s.WriteString(centerContainer.Render(menuItemStyle.Render("No downloads yet. Press '1' to switch to Add Download tab.")))
	} else {
		// Create table headers
		headers := []struct {
			content string
			width   int
		}{
			{"Path", 30},
			{"#", 5},
			{"Status", 15},
			{"Queue", 15},
			{"Progress", 10},
			{"Speed", 10},
			{"Start Time", 20},
			{"Completion Time", 20},
		}

		// Build header row
		var headerCells []string
		for _, header := range headers {
			headerCells = append(headerCells, headerStyle.Width(header.width).Render(header.content))
		}
		headerRow := lipgloss.JoinHorizontal(lipgloss.Center, headerCells...)

		// Create table rows for each download
		var rows []string
		for i, d := range m.Downloads {
			// Choose row style based on selection
			rowStyle := normalRowStyle.Copy()
			if i == m.Selected {
				rowStyle = selectedRowStyle.Copy()
			}

			// Format progress
			progress := fmt.Sprintf("%.1f%%", d.Progress)

			// Format speed
			speed := "0 B/s"
			if d.Speed > 0 {
				speed = formatSpeed(d.Speed)
			}

			// Create row cells
			cells := []struct {
				content string
				width   int
			}{
				{d.TargetPath, 30},
				{fmt.Sprintf("%d", i+1), 5},
				{d.Status, 15},
				{d.Queue, 15},
				{progress, 10},
				{speed, 10},
				{func() string {
					if !d.ScheduledStartTime.IsZero() {
						return d.ScheduledStartTime.Format("2006-01-02 15:04:05")
					}
					return d.StartTime.Format("2006-01-02 15:04:05")
				}(), 20},
				{func() string {
					if d.CompletionTime.IsZero() {
						return "-"
					}
					return d.CompletionTime.Format("2006-01-02 15:04:05")
				}(), 20},
			}

			// Build row with cells
			var rowCells []string
			for _, cell := range cells {
				rowCells = append(rowCells, rowStyle.Width(cell.width).Render(cell.content))
			}
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, rowCells...))
		}

		// Render table
		table := tableStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				headerRow,
				lipgloss.JoinVertical(lipgloss.Left, rows...),
			),
		)
		s.WriteString(centerContainer.Render(table))
	}

	// Help text
	s.WriteString("\n" + helpStyle.Width(m.Width).Render("[ ↑/↓ ] Navigate   [ p ] Pause   [ r ] Resume   [ c ] Cancel   [ y ] Retry   [ d ] Delete"))

	return s.String()
}

func renderQueueListTab(m Model) string {
	var s strings.Builder

	// Center all content
	centerContainer := centerStyle.Copy().Width(m.Width - 8)

	s.WriteString(centerContainer.Render(menuHeaderStyle.Render("Queue Management")))
	s.WriteString("\n\n")

	if m.QueueFormMode {
		// Queue form
		formContent := strings.Builder{}
		formContent.WriteString(centerContainer.Render(menuHeaderStyle.Render("Queue Configuration")))
		formContent.WriteString("\n\n")

		// Form fields with labels aligned
		labels := []string{
			"Name",
			"Path",
			"Max Concurrent",
			"Speed Limit",
			"Start Time",
			"End Time",
		}
		values := []string{
			m.InputQueueName,
			m.InputQueuePath,
			m.InputQueueConcurrent,
			m.InputQueueSpeedLimit + " KB/s (0 = unlimited)",
			m.InputQueueStartTime + " (format: HH:MM)",
			m.InputQueueEndTime + " (format: HH:MM)",
		}

		// Find the longest label for alignment
		maxLabelLen := 0
		for _, label := range labels {
			if len(label) > maxLabelLen {
				maxLabelLen = len(label)
			}
		}

		// Create a form container with proper width and centering
		formContainer := lipgloss.NewStyle().
			Width(60).
			Align(lipgloss.Center).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(CurrentTheme.Border).
			Padding(1)

		// Render form fields
		var formFields []string
		for i := range labels {
			style := queueFormFieldStyle
			if i == m.QueueFormField {
				style = queueFormSelectedFieldStyle
			}

			// Pad label to align all values
			paddedLabel := labels[i] + strings.Repeat(" ", maxLabelLen-len(labels[i]))
			formFields = append(formFields, style.Render(fmt.Sprintf("%s : %s", paddedLabel, values[i])))
		}

		// Join form fields and wrap in container
		formContent.WriteString(centerContainer.Render(
			formContainer.Render(
				lipgloss.JoinVertical(lipgloss.Center, formFields...),
			),
		))

		s.WriteString(formContent.String())
		s.WriteString("\n\n" + helpStyle.Width(m.Width).Render("[ ↑/↓ ] Navigate   [ Tab ] Next Field   [ Enter ] Save   [ Esc ] Cancel"))
	} else {
		if len(m.Config.Queues) == 0 {
			s.WriteString(centerContainer.Render(menuItemStyle.Render("No queues configured. Press 'n' to add a queue.")))
		} else {
			// Calculate table width and column widths
			tableWidth := m.Width - 24 // Account for margins, padding, and borders

			// Define proportional column widths
			nameWidth := tableWidth / 4     // 25%
			pathWidth := tableWidth * 2 / 5 // 40%
			concWidth := tableWidth / 8     // 12.5%
			speedWidth := tableWidth / 8    // 12.5%
			activeWidth := tableWidth / 10  // 10%

			// Create table container style
			tableContainer := tableStyle.Copy().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(CurrentTheme.Border).
				Padding(0, 1).
				Width(tableWidth + 4) // Add extra width for borders

			// Create header cells with borders
			headerStyle := tableHeaderStyle.Copy().
				BorderStyle(lipgloss.ThickBorder()).
				BorderBottom(true).
				BorderForeground(CurrentTheme.Highlight)

			headers := []struct {
				title string
				width int
			}{
				{"Name", nameWidth},
				{"Path", pathWidth},
				{"Max", concWidth},
				{"Speed", speedWidth},
				{"Active", activeWidth},
			}

			// Build header row
			var headerCells []string
			for _, h := range headers {
				headerCells = append(headerCells, headerStyle.Width(h.width).Render(h.title))
			}
			headerRow := lipgloss.JoinHorizontal(lipgloss.Center, headerCells...)

			// Build data rows
			var rows []string
			for i, q := range m.Config.Queues {
				// Count active downloads
				activeCount := 0
				for _, d := range m.Downloads {
					if d.Queue == q.Name && d.Status == "downloading" {
						activeCount++
					}
				}

				// Choose row style
				rowStyle := normalRowStyle.Copy()
				if i == m.QueueSelected {
					rowStyle = selectedRowStyle.Copy()
				}

				// Format speed limit
				speedLimit := "∞"
				if q.SpeedLimit > 0 {
					speedLimit = fmt.Sprintf("%dK", q.SpeedLimit)
				}

				// Create row cells
				cells := []struct {
					content string
					width   int
				}{
					{q.Name, nameWidth},
					{truncateString(q.Path, pathWidth-2), pathWidth},
					{fmt.Sprintf("%d", q.MaxConcurrent), concWidth},
					{speedLimit, speedWidth},
					{fmt.Sprintf("%d/%d", activeCount, q.MaxConcurrent), activeWidth},
				}

				// Build row with cells
				var rowCells []string
				for _, cell := range cells {
					rowCells = append(rowCells, rowStyle.Width(cell.width).Render(cell.content))
				}
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, rowCells...))
			}

			// Combine everything into the table
			table := tableContainer.Render(
				lipgloss.JoinVertical(lipgloss.Center,
					headerRow,
					lipgloss.JoinVertical(lipgloss.Center, rows...),
				),
			)

			// Center the table in the available space
			s.WriteString(centerContainer.Render(table))
		}
	}

	// Help text
	s.WriteString("\n\n" + helpStyle.Width(m.Width).Render("[ ↑/↓ ] Navigate   [ n ] New   [ e ] Edit   [ d ] Delete"))

	return s.String()
}

func renderSettingsTab(m Model) string {
	var s strings.Builder

	// Create a centered container style
	centerStyle := lipgloss.NewStyle().
		Width(m.Width - 8).
		Align(lipgloss.Center)

	s.WriteString(menuHeaderStyle.Copy().Width(m.Width - 8).Align(lipgloss.Center).Render("Settings & Help"))
	s.WriteString("\n\n")

	// Theme settings
	s.WriteString(menuHeaderStyle.Copy().Width(m.Width - 8).Align(lipgloss.Center).Render("Appearance"))
	s.WriteString("\n")
	s.WriteString(centerStyle.Render("Current Theme: " + m.CurrentTheme))
	s.WriteString("\n" + centerStyle.Render("Press 't' to cycle through available themes"))

	// Keyboard shortcuts
	s.WriteString("\n\n" + menuHeaderStyle.Copy().Width(m.Width-8).Align(lipgloss.Center).Render("Keyboard Shortcuts"))
	s.WriteString("\n")

	// Create a style for keyboard shortcuts that's centered but aligns the text internally
	shortcutStyle := lipgloss.NewStyle().
		Width(40).           // Fixed width for consistent alignment
		PaddingLeft(4).      // Add some padding to offset from center
		Align(lipgloss.Left) // Left align the text within the fixed width

	// Wrap the shortcut style in the center style
	shortcutContainer := centerStyle.Copy().
		Width(m.Width - 8).
		Align(lipgloss.Center)

	shortcuts := []string{
		"1-4:             Switch tabs",
		"↑/↓ or j/k:      Navigate lists",
		"Enter:           Confirm/Submit",
		"Esc:             Cancel/Back",
		"p:               Pause download",
		"r:               Resume download",
		"c:               Cancel download",
		"n:               New queue",
		"e:               Edit queue",
		"d:               Delete queue",
		"t:               Change theme",
		"q:               Quit application",
	}

	for _, shortcut := range shortcuts {
		s.WriteString(shortcutContainer.Render(shortcutStyle.Render(shortcut)) + "\n")
	}

	// About
	s.WriteString("\n" + menuHeaderStyle.Copy().Width(m.Width-8).Align(lipgloss.Center).Render("About"))
	s.WriteString("\n")
	s.WriteString(centerStyle.Render("Download Manager v0.1"))
	s.WriteString("\n" + centerStyle.Render("A terminal-based download manager with queue support"))

	return s.String()
}

// Helper function to truncate long strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatSpeed(speed int64) string {
	if speed < 1024 {
		return fmt.Sprintf("%d B/s", speed)
	} else if speed < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", float64(speed)/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", float64(speed)/(1024*1024))
	}
}

// Helper function to center text in a given width
func centerText(text string, width int) string {
	if width <= len(text) {
		return text
	}
	leftPadding := (width - len(text)) / 2
	return strings.Repeat(" ", leftPadding) + text
}
