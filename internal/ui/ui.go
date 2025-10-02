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

	roomInfoHomeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afff"))
	roomInfoZoneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	roomInfoOnlineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff"))
	roomInfoWatchedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd700"))
	roomInfoUptimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))

	medalStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3FB4F6")).Foreground(lipgloss.Color("#000000"))

	rankIcons = []string{"🥇", "🥈", "🥉"}
	rankStyle = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#CD7F32")),
	}

	borderStyle = lipgloss.RoundedBorder()
)

type (
	errMsg error
	App    struct {
		// 房间信息
		roomInfoBox viewport.Model
		roomInfo    model.RoomInfo

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
	}
}

func (m *App) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		listenDanmaku(),
		listenRoomInfo(),
	)
}

func (m *App) refreshRoomInfo() {
	m.roomInfoBox.SetContent(
		fmt.Sprintf("%s %s | %s %s | %s %s | %s %s | %s %v",
			roomInfoHomeStyle.Render(" ")+m.roomInfo.Title,
			roomInfoZoneStyle.Render("["+m.roomInfo.ParentAreaName+" "+m.roomInfo.AreaName+"]"),
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
		case "WATCHED_CHANGE":
			m.roomInfo.Watched = msg.Content
			m.refreshRoomInfo()
		case "ONLINE_RANK_COUNT":
			m.roomInfo.Online = msg.Content
			m.refreshRoomInfo()
		case "LIKE_INFO_V3_UPDATE":
			m.roomInfo.Liked = msg.Content
			m.refreshRoomInfo()
		default:
			var medal string
			if msg.Medal != nil {
				medal = medalStyle.Render(fmt.Sprintf("%s%2d", msg.Medal.Name, msg.Medal.Level)) + " "
			}
			m.messages.Push(fmt.Sprintf("%s %s%s %s",
				m.timeStyle.Render(msg.T.Format("[15:04]")),
				medal,
				m.senderStyle.Render(msg.Author+":"),
				msg.Content,
			))
			m.messageBox.SetContent(lipgloss.NewStyle().Width(m.messageBox.Width).Render(strings.Join(m.messages.Values(), "\n")))
			m.messageBox.GotoBottom()
		}
		cmds = append(cmds, listenDanmaku())
	case *model.RoomInfo:
		m.roomInfo.Title = msg.Title
		m.roomInfo.ParentAreaName = msg.ParentAreaName
		m.roomInfo.AreaName = msg.AreaName
		m.roomInfo.Uptime = msg.Uptime
		m.refreshRoomInfo()

		users := make([]string, len(msg.OnlineRankUsers))
		for i, u := range msg.OnlineRankUsers {
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
