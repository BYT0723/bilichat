# AGENTS.md

## Directory structure
- Client interface: `internal/client/client.go` (`Client` interface, `Message` types)
- Bilibili impl: `internal/client/bilibili/` (WebSocket, WBI signing, models, decompression)
- UI: `internal/ui/` (bubbletea model, viewports, keybindings)
- Config: `internal/config/` (YAML loading, ring-buffer history defaults)
- There is no `internal/model/` directory. Models live in `internal/client/bilibili/`.

## Commands
- Run: `make run`
- Debug: `make debug` (sets `BILICHAT_DEBUG=1`, saves raw danmaku JSON to `danmaku/`)
- With flags: `go run main.go --cookie "..." --id <room_id>`
- Build: `make build` (multi-platform static binaries to `build/`)
- Quality: `go mod tidy` only — no linter or test suite configured

## Config first-run quirk
On first run, `config.yaml` is generated as JSON content (`{"cookie": "", "room_id": 0}`) and the program exits. The file has a `.yaml` extension but contains valid JSON — both formats work.

## Build invariants
- Always `CGO_ENABLED=0` (static binaries)
- Linker flags: `-ldflags "-s -w"` (strip debug symbols)
- Go 1.24.3

## Runtime flow
1. `ui.NewApp` creates a `bilibili.Client`, calls `Start(ctx)` → WebSocket connect, goroutines: message handler, room info sync (1min), rank sync (30s), heartbeats
2. UI `listenMessage()` reads from `cli.Receive() <-chan client.Message`, returns as `tea.Msg`
3. `handleMessage` switches on `msg.Type` (`BiliBiliDanmaku`, `BiliBiliRoomInfo`, `BiliBiliRankInfo`)
