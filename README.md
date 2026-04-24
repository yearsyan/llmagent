# llmagent

Multi-backend LLM agent CLI with session-aware daemon.

Routes prompts to **Claude Code**, **OpenCode**, or **Codex** based on model ID.
Backend execution is non-interactive by default, with a background daemon
that manages sessions and streams output in real time.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/yearsyan/llmagent/main/install.sh | bash
```

Or download a binary from [releases](https://github.com/yearsyan/llmagent/releases).

## Quick start

1. Create `~/.llmagent/config.yaml`:

```yaml
models:
  claude-official:
    backend: claude-code
    official: true
    description: "Official Claude Code using your local Claude login"
  deepseek-reasoner:
    backend: claude-code
    base_url: "https://api.deepseek.com/anthropic"
    auth_token: "sk-xxx"
    model: "deepseek-reasoner"
    small_fast_model: "deepseek-chat"
  gpt-4o:
    backend: opencode
    model: "gpt-4o"

daemon:
  socket: "~/.llmagent/llmagent-dev.sock"         # On Windows: "127.0.0.1:19800"
```

2. Run a prompt:

```bash
llmagent -m deepseek-reasoner "Explain closures in JavaScript"
```

The daemon starts automatically. Each prompt creates a session you can detach
from and reattach to later.

## Usage

```
llmagent run -m <model> "prompt"   Execute prompt (daemon auto-starts)
llmagent -m <model> "prompt"       Short form of run
llmagent daemon start              Start background daemon
llmagent daemon stop               Stop daemon
llmagent daemon status             Show daemon status + all sessions
llmagent daemon attach <id>        Attach to a session (history + live)
```

## How it works

### Backends

| Backend     | Command                                 | Env vars passed                          |
|-------------|-----------------------------------------|------------------------------------------|
| claude-code with `official: true` | `claude -p "prompt" --dangerously-skip-permissions` | none |
| claude-code proxy | `claude -p "prompt" --dangerously-skip-permissions` | ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_MODEL, ANTHROPIC_SMALL_FAST_MODEL |
| opencode    | `opencode run "prompt" --model <model>` | none                                     |
| codex       | `codex exec "prompt" --skip-git-repo-check` | none                                  |

### Daemon

- Unix: Unix domain socket (`/tmp/llmagent.sock`)
- Windows: TCP loopback (`127.0.0.1:19800`)
- Auto-starts on first use, daemonized via `setsid` (survives terminal close)
- Each prompt creates a **session** with a unique ID
- Multiple terminals can attach to the same session

### Session lifecycle

```
llmagent -m model "prompt"        → [session sess-abc123]
                                     (streaming output...)
                                     Ctrl+C to detach

llmagent daemon status             → daemon: running
                                     sess-abc123  running  model  "prompt"

llmagent daemon attach sess-abc123 → (replays history + live output)
```

## Config

Config path: `$LLMAGENT_CONFIG` or `~/.llmagent/config.yaml`.

```yaml
models:
  <model-id>:
    backend: claude-code | opencode | codex
    official: true        # claude-code only; use official Claude Code login and do not inject ANTHROPIC_* env
    base_url: "..."       # claude-code proxy only; required unless official: true
    auth_token: "..."     # claude-code proxy only; required unless official: true
    model: "..."          # required for claude-code proxy; optional for opencode/codex
    small_fast_model: "..."  # optional claude-code proxy setting

daemon:
  socket: "/tmp/llmagent.sock"  # Unix: file path; Windows: "127.0.0.1:PORT"
```

## Build from source

```bash
go build -o llmagent ./cmd/llmagent/
```

Cross-compile:

```bash
GOOS=windows GOARCH=amd64 go build -o llmagent.exe ./cmd/llmagent/
GOOS=linux   GOARCH=amd64 go build -o llmagent      ./cmd/llmagent/
```
