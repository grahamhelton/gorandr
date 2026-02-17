# gorandr

A quick TUI for selecting display resolution and refresh rate on X11/Linux, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![gorandr](https://github.com/user-attachments/assets/3958b0d4-c46a-45c4-bd94-590119d7d9b6)

## Requirements

- Go 1.24+
- X11 with `xrandr`

## Install

```bash
go install github.com/grahamhelton/gorandr@latest
```

Or build from source:

```bash
git clone https://github.com/grahamhelton/gorandr.git
cd gorandr
go build -o gorandr
```

## Usage

```bash
gorandr
```

Navigate through three screens to pick your display, resolution, and refresh rate. The selected mode is applied via `xrandr`.

### Controls

| Key | Action |
|---|---|
| `j/k` or arrow keys | Navigate |
| `Enter` | Select |
| `Esc` | Back |
| `q` / `Ctrl+C` | Quit |

## Note

Only tested on a ThinkPad. If it works on your setup, great. If not, PRs welcome.
