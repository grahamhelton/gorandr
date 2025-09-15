package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("205")).
			Bold(true)

	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(2)
)

type Display struct {
	Name      string
	Current   Mode
	Available []Mode
	Connected bool
}

type Mode struct {
	Width   int
	Height  int
	Rate    float64
	Current bool
}

func (m Mode) String() string {
	marker := " "
	if m.Current {
		marker = "*"
	}
	return fmt.Sprintf("%s %dx%d @ %.1fHz", marker, m.Width, m.Height, m.Rate)
}

type Resolution struct {
	Width  int
	Height int
	Modes  []Mode
}

func (r Resolution) String() string {
	// Check if any mode with this resolution is current
	current := false
	for _, mode := range r.Modes {
		if mode.Current {
			current = true
			break
		}
	}
	marker := " "
	if current {
		marker = "*"
	}
	return fmt.Sprintf("%s %dx%d", marker, r.Width, r.Height)
}

type item struct {
	title, desc string
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }

type state int

const (
	selectingDisplay state = iota
	selectingResolution
	selectingRefreshRate
	applying
	done
)

type model struct {
	state        state
	displays     []Display
	resolutions  []Resolution
	selectedDisp int
	selectedRes  int
	list         list.Model
	message      string
	err          error
}

func initialModel() model {
	displays, err := getDisplays()
	if err != nil {
		return model{err: err}
	}

	items := make([]list.Item, len(displays))
	for i, display := range displays {
		status := "disconnected"
		if display.Connected {
			status = fmt.Sprintf("connected - %dx%d @ %.1fHz",
				display.Current.Width, display.Current.Height, display.Current.Rate)
		}
		items[i] = item{
			title: display.Name,
			desc:  status,
		}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Display"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return model{
		state:    selectingDisplay,
		displays: displays,
		list:     l,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case selectingDisplay:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "enter":
				if len(m.displays) == 0 {
					return m, tea.Quit
				}
				selected := m.list.Index()
				if !m.displays[selected].Connected {
					m.message = "Display not connected!"
					return m, nil
				}
				m.selectedDisp = selected
				m.state = selectingResolution
				m.setupResolutionList()
				return m, nil
			}
		case selectingResolution:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc":
				m.state = selectingDisplay
				m.setupDisplayList()
				return m, nil
			case "enter":
				m.selectedRes = m.list.Index()
				m.state = selectingRefreshRate
				m.setupRefreshRateList()
				return m, nil
			}
		case selectingRefreshRate:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc":
				m.state = selectingResolution
				m.setupResolutionList()
				return m, nil
			case "enter":
				rateIdx := m.list.Index()
				display := m.displays[m.selectedDisp]
				resolution := m.resolutions[m.selectedRes]
				mode := resolution.Modes[rateIdx]

				m.state = applying
				m.message = "Applying changes..."
				return m, m.applyMode(display.Name, mode)
			}
		case applying, done:
			switch msg.String() {
			case "ctrl+c", "q", "enter", "esc":
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 3)
		return m, nil

	case applyMsg:
		m.state = done
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.message = "✓ Display settings applied successfully!"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) setupDisplayList() {
	items := make([]list.Item, len(m.displays))
	for i, display := range m.displays {
		status := "disconnected"
		if display.Connected {
			status = fmt.Sprintf("connected - %dx%d @ %.1fHz",
				display.Current.Width, display.Current.Height, display.Current.Rate)
		}
		items[i] = item{
			title: display.Name,
			desc:  status,
		}
	}
	m.list.SetItems(items)
	m.list.Title = "Select Display"
}

func (m *model) setupResolutionList() {
	display := m.displays[m.selectedDisp]

	// Group modes by resolution
	resMap := make(map[string][]Mode)
	for _, mode := range display.Available {
		key := fmt.Sprintf("%dx%d", mode.Width, mode.Height)
		resMap[key] = append(resMap[key], mode)
	}

	// Convert to slice and sort
	m.resolutions = []Resolution{}
	for _, modes := range resMap {
		if len(modes) > 0 {
			// Sort modes by refresh rate (descending)
			sort.Slice(modes, func(i, j int) bool {
				return modes[i].Rate > modes[j].Rate
			})

			m.resolutions = append(m.resolutions, Resolution{
				Width:  modes[0].Width,
				Height: modes[0].Height,
				Modes:  modes,
			})
		}
	}

	// Sort resolutions by total pixels (descending)
	sort.Slice(m.resolutions, func(i, j int) bool {
		pixelsA := m.resolutions[i].Width * m.resolutions[i].Height
		pixelsB := m.resolutions[j].Width * m.resolutions[j].Height
		return pixelsA > pixelsB
	})

	items := make([]list.Item, len(m.resolutions))
	for i, resolution := range m.resolutions {
		// Show available refresh rates as description
		rates := make([]string, len(resolution.Modes))
		for j, mode := range resolution.Modes {
			rates[j] = fmt.Sprintf("%.1f", mode.Rate)
		}
		desc := fmt.Sprintf("Available rates: %s Hz", strings.Join(rates, ", "))

		items[i] = item{
			title: resolution.String(),
			desc:  desc,
		}
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Select Resolution for %s", display.Name)
}

func (m *model) setupRefreshRateList() {
	resolution := m.resolutions[m.selectedRes]
	items := make([]list.Item, len(resolution.Modes))

	for i, mode := range resolution.Modes {
		marker := " "
		if mode.Current {
			marker = "*"
		}
		title := fmt.Sprintf("%s %.1f Hz", marker, mode.Rate)
		items[i] = item{
			title: title,
			desc:  "",
		}
	}

	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Select Refresh Rate for %dx%d",
		resolution.Width, resolution.Height)
}

type applyMsg struct {
	err error
}

func (m model) applyMode(display string, mode Mode) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("xrandr", "--output", display, "--mode",
			fmt.Sprintf("%dx%d", mode.Width, mode.Height), "--rate",
			fmt.Sprintf("%.1f", mode.Rate))
		err := cmd.Run()
		return applyMsg{err: err}
	}
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress any key to exit.", m.err)
	}

	var content strings.Builder

	switch m.state {
	case selectingDisplay, selectingResolution, selectingRefreshRate:
		content.WriteString(m.list.View())
		if m.state == selectingResolution || m.state == selectingRefreshRate {
			content.WriteString("\n")
			content.WriteString(statusStyle.Render("Press 'esc' to go back"))
		}
	case applying:
		content.WriteString(titleStyle.Render("Applying Changes..."))
		content.WriteString("\n\n")
		content.WriteString(itemStyle.Render("Please wait..."))
	case done:
		content.WriteString(titleStyle.Render("Done!"))
		content.WriteString("\n\n")
		content.WriteString(itemStyle.Render(m.message))
		content.WriteString("\n\n")
		content.WriteString(statusStyle.Render("Press any key to exit"))
	}

	if m.message != "" && m.state == selectingDisplay {
		content.WriteString("\n")
		content.WriteString(statusStyle.Render(m.message))
	}

	return content.String()
}

func getDisplays() ([]Display, error) {
	cmd := exec.Command("xrandr", "--query")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run xrandr: %v", err)
	}

	return parseXrandrOutput(string(output))
}

func parseXrandrOutput(output string) ([]Display, error) {
	lines := strings.Split(output, "\n")
	var displays []Display
	var currentDisplay *Display

	// Regex patterns
	displayRe := regexp.MustCompile(`^([A-Za-z0-9\-]+)\s+(connected|disconnected)`)
	modeLineRe := regexp.MustCompile(`^\s+(\d+)x(\d+)\s+(.+)`)

	for _, line := range lines {
		if matches := displayRe.FindStringSubmatch(line); matches != nil {
			// Save previous display
			if currentDisplay != nil {
				displays = append(displays, *currentDisplay)
			}

			// Start new display
			currentDisplay = &Display{
				Name:      matches[1],
				Connected: matches[2] == "connected",
				Available: []Mode{},
			}
		} else if currentDisplay != nil && currentDisplay.Connected {
			if matches := modeLineRe.FindStringSubmatch(line); matches != nil {
				width, _ := strconv.Atoi(matches[1])
				height, _ := strconv.Atoi(matches[2])
				ratesStr := matches[3]

				// Parse all refresh rates on this line
				// Pattern: rate followed by optional * and/or +, separated by spaces
				rateRe := regexp.MustCompile(`([0-9.]+)(\*?)(\+?)`)
				rateMatches := rateRe.FindAllStringSubmatch(ratesStr, -1)

				for _, rateMatch := range rateMatches {
					rate, err := strconv.ParseFloat(rateMatch[1], 64)
					if err != nil {
						continue
					}

					current := rateMatch[2] == "*"

					mode := Mode{
						Width:   width,
						Height:  height,
						Rate:    rate,
						Current: current,
					}

					currentDisplay.Available = append(currentDisplay.Available, mode)
					if current {
						currentDisplay.Current = mode
					}
				}
			}
		}
	}
	if currentDisplay != nil {
		displays = append(displays, *currentDisplay)
	}

	return displays, nil
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
