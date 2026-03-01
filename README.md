# Vision C2

A feature-rich **Command & Control** panel built in Go, providing a fully customizable SSH-based terminal interface with MySQL backend, attack management, API funnel system, and a powerful branding/template engine.

---

## ✨ Features

- **SSH Terminal Interface** — Interactive SSH server for user sessions with custom prompts, titles, and splash screens
- **MySQL Database** — Persistent storage for users, attacks, and session data with auto-schema setup
- **Attack Management** — Multi-method attack routing with concurrency limits, cooldowns, spam protection, and blacklisting
- **API Funnel** — HTTP API endpoint for programmatic attack triggering with full validation
- **Branding Engine** — `.tfx` template system with gradient support, placeholders, and sleep directives
- **User Management** — Role-based access control (Admin / VIP / Private), plan expiration, and per-user settings
- **Logging** — File, Telegram, and Discord webhook logging support
- **Security** — bcrypt password hashing with auto-upgrade from legacy cleartext passwords

---

## 📁 Project Structure

```
vision/
├── main.go                     # Entry point — SSH + HTTP server startup
├── go.mod / go.sum             # Go module dependencies
│
├── assets/
│   ├── config.json             # Main configuration (ports, DB, slots)
│   ├── gradient.json           # Color gradient definitions for UI
│   ├── branding/               # .tfx template files for terminal UI
│   │   ├── home-splash.tfx     # Welcome screen
│   │   ├── prompt.tfx          # Command prompt
│   │   ├── title.tfx           # Terminal title bar
│   │   ├── help.tfx            # Help command output
│   │   ├── methods.tfx         # Methods list display
│   │   ├── attack-sent.tfx     # Attack confirmation message
│   │   └── ...                 # Other UI templates
│   ├── funnel/
│   │   └── funnel.json         # Attack methods & API endpoint config
│   ├── blacklists/
│   │   └── list.json           # Blocked target IPs/domains
│   └── logs/
│       └── logs.json           # Logging configuration (file/telegram/discord)
│
├── database/
│   └── Database.go             # MySQL connection, schema, CRUD operations
│
├── handlers/
│   ├── SessionHandler.go       # SSH session lifecycle management
│   ├── CommandHandler.go       # Command routing and execution
│   ├── AttackHandler.go        # Attack validation and dispatch
│   └── FunnelHandler.go        # HTTP API server and routing
│
├── managers/
│   ├── AttackManager.go        # Attack construction, API calls, ASN lookup
│   ├── FunnelManager.go        # API funnel attack processing
│   └── LogManager.go           # Multi-channel logging (file/telegram/discord)
│
├── commands/cmds/
│   ├── CommandInterface.go     # Command interface definition
│   ├── HelpCommand.go          # !help
│   ├── MethodsCommand.go       # !methods
│   ├── UsersCommand.go         # !users (admin)
│   ├── OngoingCommand.go       # !ongoing
│   ├── LogsCommand.go          # !logs (admin)
│   ├── PlanCommand.go          # !plan
│   ├── PasswordCommand.go      # !password
│   ├── ToggleCommand.go        # !toggle (admin)
│   ├── ClearCommand.go         # !clear
│   ├── EditAllCommand.go       # !editall (admin)
│   ├── CreditsCommand.go       # !credits
│   └── HelloCommand.go         # !hello
│
└── utils/
    ├── ConfigUtil.go           # Config loading and saving
    ├── BrandingUtil.go         # Template engine with gradients
    ├── MethodsUtil.go          # Method config loading and permission checks
    ├── SessionUtil.go          # Terminal output helpers
    ├── AuthenticationUtil.go   # Public IP retrieval
    ├── ExpirationUtil.go       # Expiry time formatting
    └── BlacklistUtil.go        # Blacklist file I/O
```

---

## 🛠️ Prerequisites

- **Go** 1.24+
- **MySQL** 5.7+ or MariaDB 10.3+
- **Linux** recommended for production (works on Windows/macOS for development)

---

## 🚀 Setup

### 1. Clone the Repository

```bash
git clone https://github.com/Abcl-lyxz/Vision-C2.git
cd Vision-C2
```

### 2. Configure MySQL

Create a database named `vision`:

```sql
CREATE DATABASE vision;
```

> The tables (`users`, `attacks`) are created automatically on first run.

### 3. Edit Configuration

#### `assets/config.json`

```json
{
  "cnc": {
    "port": "2222",          // SSH server port
    "api_port": "7575",      // HTTP API funnel port
    "attacks_enabled": true, // Global attack toggle
    "global_cooldown": 0,    // Cooldown between any attack (seconds, 0 = disabled)
    "global_slots": 3        // Max concurrent attacks globally
  },
  "mysql": {
    "db_user": "root",
    "db_pass": "CHANGE_ME",  // ← Set your MySQL password
    "db_host": "localhost",
    "db_name": "vision"
  }
}
```

#### `assets/funnel/funnel.json`

Configure your attack API endpoints. Each method entry has:

```json
{
  "enabled": true,
  "enabledWithFunnel": true,
  "method": "udp",
  "group": "LAYER 4",
  "defaultPort": 53,
  "defaultTime": 60,
  "minTime": 1,
  "maxTime": 400,
  "slots": 60,
  "permission": [],
  "API": [
    "http://YOUR_API:PORT/api/attack?target={host}&port={port}&time={time}&method=udp"
  ]
}
```

**Placeholders in API URLs:**
| Placeholder | Value |
|---|---|
| `{host}` or `<<$host>>` | Target IP/URL |
| `{port}` or `<<$port>>` | Target port |
| `{time}` or `<<$time>>` | Attack duration (seconds) |
| `{method}` or `<<$method>>` | Method name |

**Permission values:** `[]` = everyone, `["vip"]` = VIP only, `["ADMIN"]` = Admin only

#### `assets/logs/logs.json`

```json
{
  "global": { "enabled": true, "log_in_files": true },
  "telegram": {
    "enabled": false,
    "bot_token": "YOUR_BOT_TOKEN",
    "chat_id": "YOUR_CHAT_ID"
  },
  "discord": {
    "enabled": false,
    "webhook_url": "YOUR_DISCORD_WEBHOOK_URL"
  }
}
```

### 4. Build & Run

```bash
go build -o vision .
./vision
```

On first run:
- Database tables are created automatically
- A default `root` user is generated with a random password
- Credentials are saved to `default_user.txt`

### 5. Connect

```bash
ssh root@YOUR_SERVER_IP -p 2222
```

---

## 🎨 Branding Customization

Templates are located in `assets/branding/` as `.tfx` files.

### Available Placeholders

| Placeholder | Description |
|---|---|
| `<<$user.Username>>` | Current username |
| `<<$user.Expiry>>` | Plan expiry (human-readable) |
| `<<$user.Admin>>` | Admin status (colored true/false) |
| `<<$user.Vip>>` | VIP status |
| `<<$user.Private>>` | Private status |
| `<<$user.Concurrents>>` | Max concurrent attacks |
| `<<$user.Cooldown>>` | Cooldown (seconds) |
| `<<$user.Maxtime>>` | Max attack duration |
| `<<$user.Total_attacks>>` | Total attacks sent |
| `<<$clear>>` | Clear screen |
| `<<$sleep(1000)>>` | Pause for 1000ms |

### Gradients

Define in `assets/gradient.json`:

```json
{
  "red": {
    "from_color": "#c62828",
    "to_color": "#c0392b",
    "background": false
  }
}
```

Use in templates: `<<gradient(red)>>Your Text<<\>>`

---

## 🌐 API Endpoint

```
GET /api/attack?username=USER&password=PASS&target=IP&port=PORT&time=SECONDS&method=METHOD
```

**Response:**
```json
{
  "error": false,
  "message": {
    "target": "1.2.3.4",
    "method": "udp",
    "target_country": "US",
    "your_running_attacks": "1/3"
  }
}
```

> API access requires `api_access = 1` in the user's database record.

---

## 📋 Commands

| Command | Description | Admin |
|---|---|---|
| `help` | Show help menu | No |
| `methods` | List available attack methods | No |
| `plan` | View your account details | No |
| `ongoing` | View currently running attacks | No |
| `online` | Show online users | No |
| `password` | Change your password | No |
| `clear` | Clear terminal screen | No |
| `credits` | Show credits | No |
| `users` | User management (add/remove/edit) | Yes |
| `logs` | View/clear attack logs | Yes |
| `toggle` | Enable/disable attacks globally | Yes |
| `editall` | Bulk edit all users | Yes |

---

## 📝 License

This project is for **educational purposes only**. Use responsibly.
