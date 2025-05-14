package ui

import (
	"fmt"
	"strconv"
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
		interInfo   viewport.Model
		inputArea   textarea.Model
		senderStyle lipgloss.Style
		timeStyle   lipgloss.Style
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
	inputArea.SetHeight(1)
	// Remove cursor line styling
	inputArea.FocusedStyle.CursorLine = lipgloss.NewStyle()
	inputArea.ShowLineNumbers = false
	inputArea.KeyMap.InsertNewline.SetEnabled(false)

	interInfo := viewport.New(30, 1)
	interInfo.KeyMap = viewport.KeyMap{}

	return App{
		roomInfo:    roomInfo,
		messageBox:  messageBox,
		rankBox:     rankBox,
		giftBox:     giftBox,
		interInfo:   interInfo,
		inputArea:   inputArea,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		timeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#883388")),
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
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.inputArea, cmd = m.inputArea.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.messageBox, cmd = m.messageBox.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.roomInfo, cmd = m.roomInfo.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.rankBox, cmd = m.rankBox.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.giftBox, cmd = m.giftBox.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.roomInfo.Width = msg.Width
		rightWidth := min(40, msg.Width/2)
		m.messageBox.Width = msg.Width - rightWidth
		m.rankBox.Width = rightWidth
		m.giftBox.Width = rightWidth
		m.inputArea.SetWidth(msg.Width)
		m.interInfo.Width = msg.Width
		m.messageBox.Height = msg.Height - m.inputArea.Height() - m.roomInfo.Height - m.interInfo.Height

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
					m.messages = append(m.messages, m.senderStyle.Render("system: ")+"消息发送失败")
					m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages, "\n")))
					m.messageBox.GotoBottom()
				}
				m.inputArea.Reset()
			}
		}
	case *model.Danmaku:
		switch msg.Type {
		case "SEND_GIFT":
			m.gifts = append(m.gifts, fmt.Sprintf("%s %s",
				// m.timeStyle.Render("["+time.Now().Format(time.TimeOnly)+"]"),
				m.senderStyle.Render(msg.Author),
				msg.Content,
			))
			m.giftBox.SetContent(lipgloss.NewStyle().Width(m.giftBox.Width).Render(strings.Join(m.gifts, "\n")))
			m.giftBox.GotoBottom()
		case "INTERACT_WORD":
			m.interInfo.SetContent(fmt.Sprintf("%s %s",
				// m.timeStyle.Render("["+time.Now().Format(time.TimeOnly)+"]"),
				m.senderStyle.Render(msg.Author),
				msg.Content,
			))
		default:
			m.messages = append(m.messages, fmt.Sprintf("%s %s",
				// m.timeStyle.Render("["+time.Now().Format(time.TimeOnly)+"]"),
				m.senderStyle.Render(msg.Author+":"),
				msg.Content,
			))
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages, "\n")))
			m.messageBox.GotoBottom()
		}
		cmds = append(cmds, listenDanmaku())
	case *model.RoomInfo:
		m.roomInfo.SetContent(fmt.Sprintf("ID: %d, Name: %s, Area: %s, Online: %d, Uptime: %v", msg.RoomId, msg.Title, msg.AreaName, msg.Online, msg.Time))

		users := make([]string, len(msg.OnlineRankUsers))
		for i, u := range msg.OnlineRankUsers {
			var (
				info  = fmt.Sprintf("%d %s", u.Rank, u.Name)
				score = strconv.Itoa(int(u.Score))
			)

			spaceLen := m.rankBox.Width - lipgloss.Width(info) - lipgloss.Width(score)
			users[i] = info + strings.Repeat(" ", spaceLen) + score

		}
		m.rankBox.SetContent(strings.Join(users, "\n"))

		cmds = append(cmds, listenRoomInfo())
	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m App) View() string {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.messageBox.View(),
		lipgloss.JoinVertical(lipgloss.Top, m.giftBox.View(), m.rankBox.View()),
	)

	// 底部是输入框
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.roomInfo.View(),
		center,
		m.interInfo.View(),
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
