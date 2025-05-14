package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/BYT0723/bilichat/internal/biliclient"
	"github.com/BYT0723/bilichat/internal/config"
	"github.com/BYT0723/bilichat/internal/model"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	cli        *biliclient.Client
	danmakuCh  <-chan *model.Danmaku
	roomInfoCh <-chan *model.RoomInfo
	initOnce   sync.Once
)

type (
	errMsg error
	App    struct {
		roomInfo    viewport.Model
		messages    []string
		gifts       []string
		messageBox  viewport.Model
		rankBox     viewport.Model
		giftBox     viewport.Model
		inputArea   textarea.Model
		senderStyle lipgloss.Style
		err         error
	}
)

func NewApp() App {
	initOnce.Do(func() {
		var err error
		cli, err = biliclient.NewClient(config.Config.Cookie, uint32(config.Config.RoomId))
		if err != nil {
			panic(err)
		}
		danmakuCh, roomInfoCh = cli.Receive()
	})

	roomInfo := viewport.New(30, 1)
	roomInfo.KeyMap = viewport.KeyMap{}

	messageBox := viewport.New(30, 5)
	messageBox.KeyMap.Down.SetKeys("ctrl+n")
	messageBox.KeyMap.Up.SetKeys("ctrl+p")
	messageBox.Style = messageBox.Style.Border(lipgloss.RoundedBorder())

	rankBox := viewport.New(30, 5)
	rankBox.KeyMap = viewport.KeyMap{}
	rankBox.Style = rankBox.Style.Border(lipgloss.RoundedBorder())

	giftBox := viewport.New(30, 5)
	giftBox.KeyMap = viewport.KeyMap{}
	giftBox.Style = rankBox.Style.Border(lipgloss.RoundedBorder())

	inputArea := textarea.New()
	inputArea.Placeholder = "Send a message..."
	inputArea.Focus()
	inputArea.Prompt = "┃ "
	inputArea.CharLimit = 280
	inputArea.SetWidth(30)
	inputArea.SetHeight(2)
	// Remove cursor line styling
	inputArea.FocusedStyle.CursorLine = lipgloss.NewStyle()
	inputArea.ShowLineNumbers = false
	inputArea.KeyMap.InsertNewline.SetEnabled(false)

	return App{
		roomInfo:    roomInfo,
		messageBox:  messageBox,
		rankBox:     rankBox,
		giftBox:     giftBox,
		inputArea:   inputArea,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:         nil,
	}
}

func (m App) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		listenDanmaku(),
		listenRoomInfo(),
	)
}

func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		inputCmd, messageCmd, roomInfoCmd, rankCmd, giftCmd tea.Cmd
		cmds                                                = []tea.Cmd{inputCmd, messageCmd, roomInfoCmd, rankCmd, giftCmd}
	)

	m.inputArea, inputCmd = m.inputArea.Update(msg)
	m.messageBox, messageCmd = m.messageBox.Update(msg)
	m.roomInfo, roomInfoCmd = m.roomInfo.Update(msg)
	m.rankBox, rankCmd = m.rankBox.Update(msg)
	m.giftBox, giftCmd = m.giftBox.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.roomInfo.Width = msg.Width
		rightWidth := min(30, msg.Width/2)
		m.messageBox.Width = msg.Width - rightWidth
		m.rankBox.Width = rightWidth
		m.inputArea.SetWidth(msg.Width)
		m.messageBox.Height = msg.Height - m.inputArea.Height() - m.roomInfo.Height

		topHeight := min(10, msg.Height/2)
		m.giftBox.Height = topHeight
		m.rankBox.Height = m.messageBox.Height - topHeight

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.messageBox.GotoBottom()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			message := m.inputArea.Value()
			if len(message) > 0 {
				if err := cli.SendMsg(message); err != nil {
					m.messages = append(m.messages, m.senderStyle.Render("System: ")+"消息发送失败")
					m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages, "\n")))
					m.messageBox.GotoBottom()
				}
				m.inputArea.Reset()
			}
		}
	case *model.Danmaku:
		switch msg.Type {
		case "SEND_GIFT":
			m.gifts = append(m.messages, msg.Content)
			m.giftBox.SetContent(lipgloss.NewStyle().Width(m.giftBox.Width).Render(strings.Join(m.gifts, "\n")))
			m.giftBox.GotoBottom()
		default:
			m.messages = append(m.messages, m.senderStyle.Render(msg.Author)+": "+msg.Content)
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages, "\n")))
			m.messageBox.GotoBottom()
		}
		cmds = append(cmds, listenDanmaku())
	case *model.RoomInfo:
		m.roomInfo.SetContent(fmt.Sprintf("ID: %d, Name: %s, Area: %s, Online: %d, Uptime: %v", msg.RoomId, msg.Title, msg.AreaName, msg.Online, msg.Time))
		cmds = append(cmds, listenRoomInfo())
	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m App) View() string {
	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.messageBox.View(),
		lipgloss.JoinVertical(lipgloss.Top, m.giftBox.View(), m.rankBox.View()),
	)

	// 底部是输入框
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.roomInfo.View(),
		top,
		m.inputArea.View(),
	)
}

func listenDanmaku() tea.Cmd {
	return func() tea.Msg {
		if msg, ok := <-danmakuCh; ok {
			return msg
		}
		return nil
	}
}

func listenRoomInfo() tea.Cmd {
	return func() tea.Msg {
		if msg, ok := <-roomInfoCh; ok {
			return msg
		}
		return nil
	}
}
