package main

import (
	"flag"

	"github.com/BYT0723/bilichat/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		cookie string
		roomId int64
	)

	flag.Int64Var(&roomId, "id", 0, "room id")
	flag.StringVar(&cookie, "cookie", "", "user cookie")
	flag.Parse()

	if _, err := tea.NewProgram(ui.NewApp(cookie, int64(roomId))).Run(); err != nil {
		panic(err)
	}
}
