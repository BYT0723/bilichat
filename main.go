package main

import (
	"github.com/BYT0723/bilichat/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if _, err := tea.NewProgram(ui.NewApp()).Run(); err != nil {
		panic(err)
	}
}
