# AGENTS.md

## Directory structure

```
internal/
├── client/
│   ├── client.go            # Client interface, Message types, MessageType constants
│   └── bilibili/            # Bilibili WebSocket client implementation
│       ├── client.go        # WS connect, heartbeat, message handler goroutines
│       ├── danmaku.go       # Danmaku, Medal types, danmaku type constants
│       ├── emote.go         # Emote code → Unicode emoji mapping + ReplaceEmoteCodes()
│       ├── model.go         # handShakeInfo type
│       ├── roomInfo.go      # RoomInfo, OnlineRankUser types
│       ├── util.go          # parseCookie, brotliDecode, zlibUnCompress, splitMsg
│       └── wbi.go           # WBI signing (mixinKey, EncodeWbi)
├── config/
│   ├── config.go            # YAML loading, first-run bootstrap, logx init, ring-buffer defaults
│   ├── emote.go             # Emote struct (Disable toggle)
│   └── history.go           # History struct (Danmaku/SC/Gift ring-buffer sizes)
└── ui/
    ├── ui.go                # Bubbletea App model, handleMessage dispatch, viewports, keybindings
    ├── mode.go              # Mode type (ModeNormal, ModeInput)
    └── util.go              # FormatDurationZH, SanitizeViewportText
```

- Only Bilibili is implemented. No Douyin support exists yet.
- There is no `internal/model/` directory. Models live in `internal/client/bilibili/`.

## Commands

- Run: `make run` → `go run main.go` (uses config file, no CLI args)
- Debug: `make debug` → `BILICHAT_DEBUG=1 go run main.go` (saves raw danmaku JSON to `danmaku/`)
- With flags: `go run main.go --cookie "..." --id <room_id>` (flags override config file)
- Build: `make build` (5-platform static binaries to `build/`)
  - linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64
- Compress: `make compress` (UPX compress built binaries)
- Quality: `go mod tidy` only — no linter or test suite configured

## Build invariants

- Always `CGO_ENABLED=0` (static binaries)
- Linker flags: `-ldflags "-s -w"` (strip debug symbols)
- Go 1.24.3

## Config

- Config file at OS-appropriate path (XDG on Linux, APPDATA on Windows, ~/Library on macOS): `<config_dir>/bilichat/config.yaml`
- On first run, a YAML template is generated and the program exits with a message instructing the user to edit it:
  ```yaml
  cookie: xxx
  room_id: 0
  emote:
    disable: false
  ```
- Both YAML and JSON formats are accepted by the loader.
- Default ring-buffer sizes (set in `config.init()` if zero): Danmaku=1024, SC=512, Gift=512
- CLI flags (`--cookie`, `--id`) override config file values at runtime (merged in `ui.NewApp`).

### Emote toggle

- `emote.disable: false` (default) — replaces Bilibili emote codes like `[大哭]` with Unicode emoji (`😭[大哭]`).
- Set `emote.disable: true` to show raw emote codes only.
- Mapping defined in `internal/client/bilibili/emote.go` (~120 entries from Bilibili live room API).
- Applied in `handleMessage` before rendering DANMU_MSG content.

## Runtime flow

1. `main.go` parses `--cookie` and `--id` flags, calls `ui.NewApp(cookie, roomID)`.
2. `ui.NewApp` merges CLI flags with config (CLI wins if non-zero/non-empty), creates a `bilibili.Client`, calls `Start(ctx)`.
3. `Start(ctx)` → WebSocket connect, starts goroutines: message handler, room info sync (1min), rank sync (30s), heartbeats.
4. UI `listenMessage()` reads from `cli.Receive() <-chan client.Message`, returns as `tea.Msg`.
5. `handleMessage` switches on `msg.Type`:
   - `BiliBiliDanmaku` → type-assert to `*bilibili.Danmaku`, then sub-switch on `Danmaku.Type` string:
     - `DANMU_MSG` (default): push to messages ring-buffer (with emote replacement, medal rendering, timestamp)
     - `GUARD_BUY`, `COMBO_SEND`, `SEND_GIFT`: push to gifts ring-buffer
     - `INTERACT_WORD`, `INTERACT_WORD_V2`: set interInfo viewport (enter-room notification)
     - `SUPER_CHAT_MESSAGE`, `SUPER_CHAT_MESSAGE_JPN`: push to SC ring-buffer
     - `WATCHED_CHANGE`, `ONLINE_RANK_COUNT`, `LIKE_INFO_V3_UPDATE`: update cached RoomInfo fields
   - `BiliBiliRoomInfo` → type-assert to `*bilibili.RoomInfo`, update room header
   - `BiliBiliRankInfo` → type-assert to `[]*bilibili.OnlineRankUser`, render rank viewport

## UI modes

- `ModeNormal` (0): arrow keys / hjkl navigate viewports, `i` enters input mode
- `ModeInput` (1): textarea captures keyboard input for sending danmaku, `Esc` returns to normal

## Release

- GoReleaser CI at `.github/workflows/release.yml`, triggers on `v*` tags
- `.goreleaser.yaml` configures multi-platform builds, Arch Linux AUR, and Homebrew
