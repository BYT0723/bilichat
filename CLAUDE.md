# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BiliChat is a TUI (Terminal User Interface) for Bilibili live chat, built with [bubbletea](https://github.com/charmbracelet/bubbletea). It connects to Bilibili's live WebSocket API to display danmaku (chat messages), super chats, gifts, room information, and online rank.

## Common Commands

### Development

- `make run` - Run the application locally
- `make debug` - Run with `BILICHAT_DEBUG=1` to save raw danmaku JSON to `danmaku/` directory
- `go run main.go --cookie "..." --id <room_id>` - Run with explicit cookie and room ID

### Building

- `make build` - Build multi-platform binaries to `build/` directory (Linux x64/arm64, Windows x64, macOS x64/arm64)
- `make compress` - Compress built binaries with UPX (requires UPX installed)
- `make clean` - Remove build directory

### Release

- Releases are automated via GoReleaser on tag push (`v*`)
- Configuration: `.goreleaser.yaml`
- GitHub Actions workflow: `.github/workflows/release.yml`

### Code Quality

- `go mod tidy` - Ensure dependencies are clean
- No linting or testing suite configured
- Commit messages follow conventional style (`chore:`, `perf:`, `feat:`, `fix:`, etc.)

## Architecture

### Entry Point (`main.go`)

- Parses `--cookie` and `--id` flags, falls back to config
- Creates bubbletea program with `ui.NewApp()`

### UI Layer (`internal/ui/`)

- `ui.go` - Main Bubbletea model managing multiple viewports:
  - Danmaku messages (center)
  - Super chats (right top)
  - Gifts (right bottom)
  - Online rank (far right)
  - Room info (top)
  - User input (bottom)
- Modes: Input mode (text entry) vs Normal mode (navigation with `Ctrl+J/K`, `Ctrl+I` to enter input)
- Key bindings: Vim-style `h/j/k/l` for viewport navigation in active pane
- Utilities (`util.go`): Chinese duration formatting (`FormatDurationZH`), sanitization of zero-width/control characters (`SanitizeViewportText`)

### Bilibili Client (`internal/biliclient/`)

- `client.go` - WebSocket client with authentication, message parsing, heartbeats
- Connects to Bilibili's live Danmaku server via WSS using WBI signed requests (`wbi.go`)
- Parses protocol buffer and JSON messages into `model.Danmaku` using `gjson`
- Utilities (`util.go`): cookie parsing, brotli/zlib decompression, message splitting
- Separate goroutines for:
  - WebSocket message handling (`handlerMsg`)
  - Room info sync every minute (`syncRoomInfo`)
  - Online rank sync every 30 seconds (`syncRank`)
  - Connection heartbeats every 30 seconds (`connHeartBeat`)
  - Video heartbeats every 10 seconds (`videoHeartBeat`)
- Channels: `msgCh`, `roomInfoCh`, `rankCh` feed into UI
- Uses ring buffers (`github.com/BYT0723/go-tools/ds`) for history management

### Data Models (`internal/model/`)

- `Danmaku` - Chat message with author, content, type, timestamp, optional medal
- `RoomInfo` - Room metadata (title, streamer, area, viewer counts, uptime)
- `OnlineRankUser` - Top contributors with score and rank

### Configuration (`internal/config/`)

- `config.go` - Loads YAML config from platform-specific directory:
  - Linux: `~/.config/bilichat/config.yaml`
  - macOS: `~/Library/Application Support/bilichat/config.yaml`
  - Windows: `%APPDATA%\bilichat\config.yaml`
- First run generates template with empty `cookie` and `room_id`
- `History` struct configures ring buffer sizes for danmaku, SC, and gift history

## Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/iyear/biligo` - Bilibili API client
- `github.com/gorilla/websocket` - WebSocket implementation
- `github.com/BYT0723/go-tools` - Internal utilities (logging, HTTP, ring buffer)
- `github.com/tidwall/gjson` - JSON parsing

## Configuration File

Example `config.yaml`:

```yaml
cookie: "SESSDATA=...; DedeUserID=..."
room_id: 123456
history:
  danmaku: 1024
  sc: 512
  gift: 512
```

## Environment Variables

- `BILICHAT_DEBUG=1` - Save raw danmaku JSON messages to `danmaku/` directory for debugging

## Build Details

- Go 1.24.3
- CGO disabled for static binaries
- Build flags: `-s -w` to strip debug symbols
- Multi-architecture builds via Makefile

## Project Structure

```
.
├── main.go                 # Entry point
├── Makefile               # Build commands
├── .goreleaser.yaml      # Release configuration
├── internal/
│   ├── ui/               # TUI components
│   ├── biliclient/       # Bilibili WebSocket client
│   ├── model/           # Data structures
│   └── config/          # Configuration loading
└── .github/workflows/    # CI/CD
```

# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
