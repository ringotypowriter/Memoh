# config.toml Reference

Memoh uses a TOML configuration file. By default it looks for `config.toml` in the current directory. With Docker, you can mount a custom config via `MEMOH_CONFIG` (see [Docker installation](./docker#custom-configuration)).

## Full Example

```toml
[log]
level = "info"
format = "text"

[server]
addr = ":8080"

[admin]
username = "admin"
password = "change-your-password"
email = "admin@example.com"

[auth]
jwt_secret = "your-secret-from-openssl-rand-base64-32"
jwt_expires_in = "168h"

[containerd]
socket_path = "/run/containerd/containerd.sock"
namespace = "default"

[mcp]
image = "docker.io/library/memoh-mcp:latest"
snapshotter = "overlayfs"
data_root = "data"

[postgres]
host = "127.0.0.1"
port = 5432
user = "postgres"
password = "your-password"
database = "memoh"
sslmode = "disable"

[qdrant]
base_url = "http://127.0.0.1:6334"
api_key = ""
collection = "memory"
timeout_seconds = 10

[agent_gateway]
host = "127.0.0.1"
port = 8081

[web]
host = "127.0.0.1"
port = 8082

[brave]
api_key = ""
base_url = "https://api.search.brave.com/res/v1/"
```

## Section Reference

### `[log]`

| Field   | Type   | Default | Description                                      |
|---------|--------|---------|--------------------------------------------------|
| `level` | string | `"info"` | Log level: `debug`, `info`, `warn`, `error`     |
| `format`| string | `"text"` | Log format: `text` or `json`                    |

### `[server]`

| Field  | Type   | Default | Description                                      |
|--------|--------|---------|--------------------------------------------------|
| `addr` | string | `":8080"` | HTTP listen address. Use `:8080` for all interfaces, or `host:port` (e.g. `server:8080` in Docker). |

### `[admin]`

| Field      | Type   | Default | Description                          |
|------------|--------|---------|--------------------------------------|
| `username` | string | `"admin"` | Admin login username                |
| `password` | string | —       | Admin login password. **Change in production.** |
| `email`    | string | —       | Admin email (for display)            |

### `[auth]`

| Field          | Type   | Default | Description                                      |
|----------------|--------|---------|--------------------------------------------------|
| `jwt_secret`   | string | —       | Secret for signing JWT tokens. **Required.** Generate with `openssl rand -base64 32`. |
| `jwt_expires_in` | string | `"24h"` | JWT expiration, e.g. `"24h"`, `"168h"` (7 days) |

### `[containerd]`

| Field         | Type   | Default | Description                                      |
|---------------|--------|---------|--------------------------------------------------|
| `socket_path` | string | `"/run/containerd/containerd.sock"` | Path to containerd socket       |
| `namespace`   | string | `"default"` | Containerd namespace for bot containers      |

### `[mcp]`

MCP (Model Context Protocol) container configuration. Each bot runs in a container built from this image.

| Field         | Type   | Default | Description                                      |
|---------------|--------|---------|--------------------------------------------------|
| `image`       | string | `"docker.io/library/memoh-mcp:latest"` | MCP container image        |
| `snapshotter` | string | `"overlayfs"` | Containerd snapshotter                      |
| `data_root`   | string | `"data"` | Host path for bot data (Docker: `/opt/memoh/data`) |

### `[postgres]`

| Field     | Type   | Default | Description                                      |
|-----------|--------|---------|--------------------------------------------------|
| `host`    | string | `"127.0.0.1"` | PostgreSQL host                             |
| `port`    | int    | `5432`  | PostgreSQL port                                  |
| `user`    | string | `"postgres"` | Database user                               |
| `password`| string | —       | Database password                                |
| `database`| string | `"memoh"` | Database name                                 |
| `sslmode` | string | `"disable"` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

### `[qdrant]`

| Field            | Type   | Default | Description                                      |
|------------------|--------|---------|--------------------------------------------------|
| `base_url`       | string | `"http://127.0.0.1:6334"` | Qdrant HTTP API URL                    |
| `api_key`        | string | `""`    | Optional API key for Qdrant Cloud                 |
| `collection`     | string | `"memory"` | Vector collection name for memories           |
| `timeout_seconds`| int    | `10`    | Request timeout in seconds                       |

### `[agent_gateway]`

| Field  | Type   | Default | Description                                      |
|--------|--------|---------|--------------------------------------------------|
| `host` | string | `"127.0.0.1"` | Agent gateway bind host                       |
| `port` | int    | `8081`  | Agent gateway port                               |

In Docker Compose, `host` is typically `"agent"` (service name). The agent reads `[server].addr` to call the main API.

### `[web]`

| Field  | Type   | Default | Description                                      |
|--------|--------|---------|--------------------------------------------------|
| `host` | string | `"127.0.0.1"` | Web UI bind host                              |
| `port` | int    | `8082`  | Web UI port                                      |

### `[brave]`

Brave Search API for the web search tool. Leave `api_key` empty to disable web search.

| Field     | Type   | Default | Description                                      |
|-----------|--------|---------|--------------------------------------------------|
| `api_key` | string | `""`    | Brave Search API key. Get one at [brave.com/search/api](https://brave.com/search/api). |
| `base_url`| string | `"https://api.search.brave.com/res/v1/"` | Brave Search API base URL          |
