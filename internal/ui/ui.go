package ui

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/BYT0723/bilichat/internal/client"
	"github.com/BYT0723/bilichat/internal/client/bilibili"
	"github.com/BYT0723/bilichat/internal/config"
	"github.com/BYT0723/go-tools/ds"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	cli client.Client

	roomInfoHomeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afff"))
	roomInfoZoneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	roomInfoOnlineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff"))
	roomInfoWatchedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd700"))
	roomInfoUptimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))

	medalStyle      = lipgloss.NewStyle().Background(lipgloss.Color("#3FB4F6")).Foreground(lipgloss.Color("#000000"))
	medalLevelStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3FB4F6")).Foreground(lipgloss.Color("#000000")).Bold(true)

	rankIcons = []string{"🥇", "🥈", "🥉"}
	rankStyle = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#CD7F32")),
	}

	defaultKeyMap = viewport.KeyMap{
		Up:    key.NewBinding(key.WithKeys("k")),
		Down:  key.NewBinding(key.WithKeys("j")),
		Left:  key.NewBinding(key.WithKeys("h")),
		Right: key.NewBinding(key.WithKeys("l")),
	}

	normalBorderStyle = lipgloss.RoundedBorder()
	activeBorderStyle = lipgloss.DoubleBorder()

	modelIndexes = []string{"danmaku", "sc", "gift", "rank"}
)

type (
	errMsg error
	App    struct {
		// 房间信息
		roomInfoBox viewport.Model
		roomInfo    bilibili.RoomInfo

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

		index int

		// 当前模式
		// 0 0 0 0 0 0 0 1		插入模式
		mode Mode
	}
)

func NewApp(cookie string, roomID int64) *App {
	var err error
	cookie = cmp.Or(cookie, config.Config.Cookie)
	roomID = cmp.Or(roomID, config.Config.RoomID)

	cli, err = bilibili.NewClient(cookie, uint32(roomID))
	if err != nil {
		panic(err)
	}

	if err := cli.Start(context.Background()); err != nil {
		panic(err)
	}

	roomInfo := viewport.New(30, 1)
	roomInfo.KeyMap = viewport.KeyMap{}

	messageBox := viewport.New(30, 5)
	messageBox.KeyMap = defaultKeyMap
	messageBox.Style = messageBox.Style.Border(normalBorderStyle)

	scBox := viewport.New(30, 5)
	scBox.KeyMap = viewport.KeyMap{}
	scBox.Style = scBox.Style.Border(normalBorderStyle)

	rankBox := viewport.New(30, 5)
	rankBox.KeyMap = viewport.KeyMap{}
	rankBox.Style = rankBox.Style.Border(normalBorderStyle)

	giftBox := viewport.New(30, 5)
	giftBox.KeyMap = viewport.KeyMap{}
	giftBox.Style = rankBox.Style.Border(normalBorderStyle)

	inputArea := textarea.New()
	inputArea.Placeholder = "say something..."
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

	app := &App{
		roomInfoBox: roomInfo,
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
		timeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#545c7e")),
		err:         nil,
		mode:        ModeInput,
	}

	return app
}

func (m *App) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, listenMessage)
}

func (m *App) refreshRoomInfo() {
	m.roomInfoBox.SetContent(
		fmt.Sprintf("%s %s %s | %s %s | %s %s | %s %s | %s %v",
			roomInfoHomeStyle.Render("  ")+m.roomInfo.Title,
			roomInfoZoneStyle.Render("["+m.roomInfo.ParentAreaName+" "+m.roomInfo.AreaName+"]"),
			m.roomInfo.Uname,
			roomInfoWatchedStyle.Render(" "), m.roomInfo.Watched,
			roomInfoOnlineStyle.Render(" "), m.roomInfo.Liked,
			roomInfoOnlineStyle.Render(""), m.roomInfo.Online,
			roomInfoUptimeStyle.Render(" "), FormatDurationZH(m.roomInfo.Uptime/time.Minute*time.Minute),
		),
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
	m.roomInfoBox, cmd = m.roomInfoBox.Update(msg)
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
		m.roomInfoBox.Width = msg.Width

		rightWidth := min(40, msg.Width/2)
		topHeight := min(10, msg.Height/2)

		m.inputArea.SetWidth(msg.Width)

		m.interInfo.Width = msg.Width

		m.messageBox.Width = (msg.Width - 2*rightWidth)
		m.messageBox.Height = msg.Height - m.inputArea.Height() - m.roomInfoBox.Height - m.interInfo.Height

		m.scBox.Width = rightWidth
		m.scBox.Height = topHeight

		m.giftBox.Width = rightWidth
		m.giftBox.Height = m.messageBox.Height - topHeight

		m.rankBox.Width = rightWidth
		m.rankBox.Height = m.messageBox.Height

		if m.messages.Len() > 0 {
			// Wrap content before setting it.
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
			m.scBox.SetContent(lipgloss.NewStyle().Width(m.scBox.Width).Render(strings.Join(m.sc.Values(), "\n")))
		}
		if m.mode == ModeInput {
			m.messageBox.GotoBottom()
			m.scBox.GotoBottom()
		}
	case tea.KeyMsg:
		if subCmd := m.handleKeyMap(msg); subCmd != nil {
			return m, subCmd
		}
	case client.Message:
		if subcmds := m.handleMessage(msg); len(subcmds) > 0 {
			cmds = append(cmds, subcmds...)
		}
		cmds = append(cmds, listenMessage)
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
		lipgloss.JoinVertical(lipgloss.Top, m.scBox.View(), m.giftBox.View()),
		m.rankBox.View(),
	)

	// 底部是输入框
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.roomInfoBox.View(),
		center,
		m.interInfo.View(),
		m.inputArea.View(),
	)
}

func (m *App) handleKeyMap(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyCtrlC:
		return tea.Quit

	case tea.KeyEsc:
		if m.mode == ModeInput {
			m.inputArea.Blur()
			m.mode = ModeNormal
		}

	case tea.KeyCtrlI:
		if m.mode == ModeNormal {
			switch modelIndexes[m.index] {
			case "danmaku":
				m.messageBox.Style = m.messageBox.Style.Border(normalBorderStyle)
				m.messageBox.KeyMap = viewport.KeyMap{}
			case "sc":
				m.scBox.Style = m.scBox.Style.Border(normalBorderStyle)
				m.scBox.KeyMap = viewport.KeyMap{}
			case "gift":
				m.giftBox.Style = m.giftBox.Style.Border(normalBorderStyle)
				m.giftBox.KeyMap = viewport.KeyMap{}
			case "rank":
				m.rankBox.Style = m.rankBox.Style.Border(normalBorderStyle)
				m.rankBox.KeyMap = viewport.KeyMap{}
			}
			m.inputArea.Focus()
			m.mode = ModeInput
		}

	case tea.KeyCtrlJ, tea.KeyCtrlK:
		if m.mode == ModeNormal {
			switch modelIndexes[m.index] {
			case "danmaku":
				m.messageBox.Style = m.messageBox.Style.Border(normalBorderStyle)
				m.messageBox.KeyMap = viewport.KeyMap{}
			case "sc":
				m.scBox.Style = m.scBox.Style.Border(normalBorderStyle)
				m.scBox.KeyMap = viewport.KeyMap{}
			case "gift":
				m.giftBox.Style = m.giftBox.Style.Border(normalBorderStyle)
				m.giftBox.KeyMap = viewport.KeyMap{}
			case "rank":
				m.rankBox.Style = m.rankBox.Style.Border(normalBorderStyle)
				m.rankBox.KeyMap = viewport.KeyMap{}
			}
			switch msg.Type {
			case tea.KeyCtrlJ:
				m.index = (m.index + 1) % len(modelIndexes)
			case tea.KeyCtrlK:
				m.index = (m.index - 1 + len(modelIndexes)) % len(modelIndexes)
			}
		}

		if m.mode == ModeInput {
			m.inputArea.Blur()
			m.mode = ModeNormal
		}

		switch modelIndexes[m.index] {
		case "danmaku":
			m.messageBox.Style = m.messageBox.Style.Border(activeBorderStyle)
			m.messageBox.KeyMap = defaultKeyMap
		case "sc":
			m.scBox.Style = m.scBox.Style.Border(activeBorderStyle)
			m.scBox.KeyMap = defaultKeyMap
		case "gift":
			m.giftBox.Style = m.giftBox.Style.Border(activeBorderStyle)
			m.giftBox.KeyMap = defaultKeyMap
		case "rank":
			m.rankBox.Style = m.rankBox.Style.Border(activeBorderStyle)
			m.rankBox.KeyMap = defaultKeyMap
		}

	case tea.KeyEnter:
		switch m.mode {
		case ModeInput:
			message := m.inputArea.Value()
			if len(message) > 0 {
				if err := cli.Send(message); err != nil {
					m.messages.Push(m.senderStyle.Render("system: ") + "消息发送失败")
					m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
					m.messageBox.GotoBottom()
				}
				m.inputArea.Reset()
			}
		}
	}
	return nil
}

func (m *App) handleMessage(msg client.Message) (cmds []tea.Cmd) {
	switch msg.Type {
	case client.BiliBiliDanmaku:
		v, ok := msg.Data.(*bilibili.Danmaku)
		if ok {
			switch v.Type {
			case "GUARD_BUY", "COMBO_SEND", "SEND_GIFT":
				m.gifts.Push(fmt.Sprintf("%s %s", m.senderStyle.Render(v.Author), v.Content))
				m.giftBox.SetContent(lipgloss.NewStyle().Width(m.giftBox.Width).Render(strings.Join(m.gifts.Values(), "\n")))
				if m.mode == ModeInput {
					m.giftBox.GotoBottom()
				}
			case "INTERACT_WORD":
				m.interInfo.SetContent(fmt.Sprintf("%s %s", m.senderStyle.Render(v.Author), v.Content))
			case "INTERACT_WORD_V2":
				m.interInfo.SetContent(fmt.Sprintf("%s %s", m.senderStyle.Render(v.Author), v.Content))
			case "SUPER_CHAT_MESSAGE", "SUPER_CHAT_MESSAGE_JPN":
				m.sc.Push(fmt.Sprintf("%s %s", m.senderStyle.Render(v.Author+":"), v.Content))
				m.scBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.sc.Values(), "\n")))
				if m.mode == ModeInput {
					m.scBox.GotoBottom()
				}
			case "WATCHED_CHANGE":
				m.roomInfo.Watched = v.Content
				m.refreshRoomInfo()
			case "ONLINE_RANK_COUNT":
				m.roomInfo.Online = v.Content
				m.refreshRoomInfo()
			case "LIKE_INFO_V3_UPDATE":
				m.roomInfo.Liked = v.Content
				m.refreshRoomInfo()
			default:
				var medal string
				if v.Medal != nil {
					medal = medalStyle.Render(v.Medal.Name+" ") + medalLevelStyle.Render(fmt.Sprintf("%2d", v.Medal.Level)) + " "
				}
				author := SanitizeViewportText(v.Author)
				content := SanitizeViewportText(v.Content)
				m.messages.Push(fmt.Sprintf("%s %s%s %s",
					m.timeStyle.Render(v.T.Format("[15:04]")),
					medal,
					m.senderStyle.Render(author+":"),
					content,
				))
				m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
				if m.mode == ModeInput {
					m.messageBox.GotoBottom()
				}
			}
		}
	case client.BiliBiliRankInfo:
		v, ok := msg.Data.([]*bilibili.OnlineRankUser)
		if ok {

			users := make([]string, len(v))
			for i, u := range v {
				var (
					t     = "  "
					score = strconv.Itoa(int(u.Score))
				)
				if int(u.Rank) <= len(rankIcons) {
					t = rankStyle[u.Rank-1].Render(rankIcons[u.Rank-1])
				}
				info := fmt.Sprintf("%s %s", t, u.Name)

				spaceLen := m.rankBox.Width - lipgloss.Width(info) - lipgloss.Width(score) - m.rankBox.Style.GetHorizontalBorderSize()
				users[i] = info + strings.Repeat(" ", spaceLen) + score

			}
			m.rankBox.SetContent(strings.Join(users, "\n"))
		}
	case client.BiliBiliRoomInfo:
		v, ok := msg.Data.(*bilibili.RoomInfo)
		if ok {
			m.roomInfo.Title = v.Title
			m.roomInfo.Uname = v.Uname
			m.roomInfo.ParentAreaName = v.ParentAreaName
			m.roomInfo.AreaName = v.AreaName
			m.roomInfo.Uptime = v.Uptime
			m.refreshRoomInfo()
		}
	}
	return
}

func listenMessage() tea.Msg {
	if msg, ok := <-cli.Receive(); ok {
		return msg
	}
	return nil
}
