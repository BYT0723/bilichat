package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BYT0723/bilichat/internal/biliclient"
	"github.com/BYT0723/bilichat/internal/config"
	"github.com/BYT0723/bilichat/internal/model"
	"github.com/BYT0723/go-tools/ds"
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

	roomInfoHomeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afff"))
	roomInfoZoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	roomInfoUserStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff"))
	roomInfoStarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd700"))
	roomInfoUptimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))

	borderStyle = lipgloss.RoundedBorder()
)

type (
	errMsg error
	App    struct {
		// 房间信息
		roomInfo viewport.Model

		// sc 醒目留言
		sc    *ds.RingBuffer[string]
		scBox viewport.Model

		// 弹幕
		messages    *ds.RingBuffer[string]
		messageBox  viewport.Model
		senderStyle lipgloss.Style

		// 礼物
		gifts   *ds.RingBuffer[string]
		giftBox viewport.Model

		// 打榜
		rankBox viewport.Model

		// 进房
		interInfo viewport.Model

		// 输入
		inputArea textarea.Model

		timeStyle lipgloss.Style
		err       error
	}
)

func NewApp(cookie string, roomId int64) *App {
	if cookie == "" {
		cookie = config.Config.Cookie
	}
	if roomId == 0 {
		roomId = config.Config.RoomID
	}
	initOnce.Do(func() {
		var err error
		cli, err = biliclient.NewClient(cookie, uint32(roomId))
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
	messageBox.Style = messageBox.Style.Border(borderStyle)

	scBox := viewport.New(30, 5)
	scBox.KeyMap.Down.SetKeys("ctrl+n")
	scBox.KeyMap.Up.SetKeys("ctrl+p")
	scBox.Style = scBox.Style.Border(borderStyle)

	rankBox := viewport.New(30, 5)
	rankBox.KeyMap = viewport.KeyMap{}
	rankBox.Style = rankBox.Style.Border(borderStyle)

	giftBox := viewport.New(30, 5)
	giftBox.KeyMap = viewport.KeyMap{}
	giftBox.Style = rankBox.Style.Border(borderStyle)

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

	return &App{
		roomInfo:    roomInfo,
		messages:    ds.NewRingBufferWithSize[string](config.Config.History.Danmaku),
		messageBox:  messageBox,
		sc:          ds.NewRingBufferWithSize[string](config.Config.History.SC),
		scBox:       scBox,
		rankBox:     rankBox,
		gifts:       ds.NewRingBufferWithSize[string](config.Config.History.Gift),
		giftBox:     giftBox,
		interInfo:   interInfo,
		inputArea:   inputArea,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		timeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#883388")),
		err:         nil,
	}
}

func (m *App) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		listenDanmaku(),
		listenRoomInfo(),
	)
}

func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.messageBox.Width = (msg.Width - rightWidth) / 2
		m.scBox.Width = m.messageBox.Width
		m.rankBox.Width = rightWidth
		m.giftBox.Width = rightWidth
		m.inputArea.SetWidth(msg.Width)
		m.interInfo.Width = msg.Width
		m.messageBox.Height = msg.Height - m.inputArea.Height() - m.roomInfo.Height - m.interInfo.Height
		m.scBox.Height = m.messageBox.Height

		topHeight := min(10, msg.Height/2)
		m.giftBox.Height = topHeight
		m.rankBox.Height = m.messageBox.Height - topHeight

		if m.messages.Len() > 0 {
			// Wrap content before setting it.
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
			m.scBox.SetContent(lipgloss.NewStyle().Width(m.scBox.Width).Render(strings.Join(m.sc.Values(), "\n")))
		}
		m.messageBox.GotoBottom()
		m.scBox.GotoBottom()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			message := m.inputArea.Value()
			if len(message) > 0 {
				if err := cli.SendMsg(message); err != nil {
					m.messages.Push(m.senderStyle.Render("system: ") + "消息发送失败")
					m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
					m.messageBox.GotoBottom()
				}
				m.inputArea.Reset()
			}
		}
	case *model.Danmaku:
		switch msg.Type {
		case "SEND_GIFT":
			m.gifts.Push(fmt.Sprintf("%s %s", m.senderStyle.Render(msg.Author), msg.Content))
			m.giftBox.SetContent(lipgloss.NewStyle().Width(m.giftBox.Width).Render(strings.Join(m.gifts.Values(), "\n")))
			m.giftBox.GotoBottom()
		case "INTERACT_WORD":
			m.interInfo.SetContent(fmt.Sprintf("%s %s", m.senderStyle.Render(msg.Author), msg.Content))
		case "INTERACT_WORD_V2":
			m.interInfo.SetContent(fmt.Sprintf("%s %s", m.senderStyle.Render(msg.Author), msg.Content))
		case "SUPER_CHAT_MESSAGE", "SUPER_CHAT_MESSAGE_JPN":
			m.sc.Push(fmt.Sprintf("%s %s", m.senderStyle.Render(msg.Author+":"), msg.Content))
			m.scBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.sc.Values(), "\n")))
			m.scBox.GotoBottom()
		default:
			m.messages.Push(fmt.Sprintf("%s %s", m.senderStyle.Render(msg.Author+":"), msg.Content))
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
			m.messageBox.GotoBottom()
		}
		cmds = append(cmds, listenDanmaku())
	case *model.RoomInfo:
		m.roomInfo.SetContent(
			fmt.Sprintf("%s %s | %s %d | %s %d | %s %v",
				roomInfoHomeStyle.Render(" ")+msg.Title,
				roomInfoZoneStyle.Render("["+msg.ParentAreaName+" "+msg.AreaName+"]"),
				roomInfoUserStyle.Render(""), msg.Online,
				roomInfoStarStyle.Render(""), msg.Attention,
				roomInfoUptimeStyle.Render(""), FormatDurationZH(msg.Uptime/time.Minute*time.Minute),
			),
		)

		users := make([]string, len(msg.OnlineRankUsers))
		for i, u := range msg.OnlineRankUsers {
			var (
				icons = []string{"🥇", "🥈", "🥉"}
				t     = " "
				score = strconv.Itoa(int(u.Score))
			)
			if int(u.Rank) <= len(icons) {
				t = icons[u.Rank-1]
			}
			info := fmt.Sprintf("%s %s", t, u.Name)

			spaceLen := m.rankBox.Width - lipgloss.Width(info) - lipgloss.Width(score) - m.rankBox.Style.GetHorizontalBorderSize()
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

func (m *App) View() string {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.messageBox.View(),
		m.scBox.View(),
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
