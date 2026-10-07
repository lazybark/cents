# Running cents-server on fly.io

Status: **an idea, not built yet.** For now the server runs on a laptop.
This note covers what hosting it on fly.io would take, so the work can be
picked up later.

## The setup in short

```
desktop app ──HTTPS──▶ Fly edge (TLS, cents.example.com)
                           │ private network, plain HTTP
                           ▼
                   cents-server (one machine)
                           │
                   SQLite on a Fly Volume ──Litestream──▶ S3-compatible bucket
```

- **Desktop app:** nothing changes. Use Settings → Database → Connect to a
  server… with `https://cents.example.com` and a token. Fly gives the domain
  a Let's Encrypt certificate, so the system trusts it and the app doesn't
  ask about a fingerprint.
- **Database:** the SQLite file lives on a **Fly Volume** (a disk attached to
  the machine), not in S3.
- **Copies:** **Litestream** streams every change to the S3 bucket within
  about a second, and restores the file from it if the volume is lost.

### Why not keep the database file in S3

SQLite needs a real filesystem with working file locks, and object storage
isn't one. Mounting a bucket as a drive (s3fs and the like) is slow, has no
real locking, and risks corrupting the database. Litestream is how SQLite is
normally paired with S3: the disk keeps the live file and the bucket keeps
the copies.

## What has to change in the code

### 1. Real client addresses behind the proxy (required)

Fly's edge ends HTTPS and forwards requests to the machine, so every request
reaches cents-server from the proxy's address. The server deliberately
ignores headers like `X-Forwarded-For`, because a client can send them
(`remoteAddr` in `server/middleware.go`). Behind the proxy that breaks two
things:

- **Allowlist (`allow`):** it would have to include Fly's internal network,
  which then lets everyone in. The token still guards the API.
- **Failed-login throttle (`throttle` in `server/auth.go`):** it counts
  failures per address. Every client would share the proxy's address, so 10
  bad tokens from anyone would lock everyone out for a minute, and a stranger
  could keep doing it.

Fix: a `trusted_proxies` config key (a list of CIDRs, empty by default).
Only when a connection comes from one of those addresses, the server takes
the client's address from Fly's `Fly-Client-IP` header (or the last
`X-Forwarded-For` entry). The allowlist, the throttle and the logs then use
that address. Connections from anywhere else keep ignoring the headers.

Check the exact source range of Fly's proxy when building this. It's
expected to be Fly's private IPv6 network (`fdaa::/16`), but confirm it from
the machine.

### 2. WAL mode, for Litestream (required)

Litestream works only with SQLite in WAL mode, and `OpenDatabase` in
`storage/sqlite/storage.go` doesn't set a journal mode today. Add
`PRAGMA journal_mode=WAL` when opening a file-backed database. It's also a
good change for the desktop app.

Things to check:

- **Backups:** the `-wal` and `-shm` files next to the database. VACUUM INTO
  already writes one consistent file.
- **Encryption:** the vault (`storage/sqlite/vault.go`) runs in memory and
  doesn't need WAL.
- **Moving or copying the database:** checkpoint first, since the `-wal`
  file holds recent changes.

### 3. Timezone (recommended)

Fly machines run on UTC. "Today" and "this month" (budgets, overdue
payments, the running month) follow the server's clock. Set `TZ` to the
user's zone in `fly.toml`, and build the server with Go's embedded zone data
(`-tags timetzdata`, or import `time/tzdata` in `cmd/server`), because slim
images ship without `/usr/share/zoneinfo`.

### 4. Encrypted databases (limitation)

The server can't open password-protected databases yet (`Run` in
`server/server.go` refuses them), so the file on the volume must be plain.
Litestream can encrypt the copies it uploads (its `age` support), so the
bucket stays private even if its credentials leak.

## Deployment files to write

None of these exist yet. They are rough sketches to check against the
current Fly and Litestream docs.

### Dockerfile (two-stage: the SQLite driver needs cgo)

```dockerfile
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -tags timetzdata -o /cents-server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /cents-server /usr/local/bin/cents-server
# Add the litestream binary from its release (or the litestream image).
COPY litestream.yml /etc/litestream.yml
# Litestream restores the database if it's missing, then runs the server
# as its child process and replicates its changes.
CMD ["litestream", "replicate", "-exec", "cents-server --config /data/server.yml"]
```

On first boot, run `litestream restore -if-db-not-exists -if-replica-exists`
before `replicate` (for example in a small entrypoint script).

### fly.toml

```toml
app = "cents"
primary_region = "ams"            # nearest to you

[env]
  TZ = "Europe/Amsterdam"         # your zone

[mounts]
  source = "cents_data"
  destination = "/data"

[http_service]
  internal_port = 8740
  force_https = true
  auto_stop_machines = "stop"     # waking takes a few seconds; the app waits 15
  auto_start_machines = true
  min_machines_running = 0
```

Keep the app on **one machine** (`fly scale count 1`). SQLite allows a
single writer, and a volume belongs to one machine.

### server.yml (on the volume, at /data/server.yml)

```yaml
listen: 0.0.0.0:8740
database: /data/cents.db
# HTTPS ends at Fly's edge; this hop runs on Fly's private network.
insecure_http: true
# Needs change 1 above:
# trusted_proxies: [fdaa::/16]
# allow: [...]   # once real addresses are seen, if wanted
tokens:
  - name: laptop
    sha256: ...
log:
  format: json
```

Keep tokens out of the image. Make them with
`cents-server token new --name laptop` and put the config on the volume, or
build it from Fly secrets at start.

### litestream.yml

```yaml
dbs:
  - path: /data/cents.db
    replicas:
      - type: s3
        bucket: cents-backups
        path: cents.db
        endpoint: https://...     # any S3-compatible store (Tigris on Fly, R2, B2, ...)
        # access keys from env: LITESTREAM_ACCESS_KEY_ID / LITESTREAM_SECRET_ACCESS_KEY (Fly secrets)
```

## Getting the existing data there

1. Stop the desktop app (or the laptop server) so the database isn't
   changing.
2. Copy the database to the volume: `fly ssh sftp shell`, then `put`. Or
   restore it through Litestream after uploading it once.
3. Start the app on Fly. In the desktop app, connect to
   `https://cents.example.com` with a new token.

## Checklist

- [ ] `trusted_proxies` + `Fly-Client-IP` (server/middleware.go, config.go), with tests
- [ ] WAL mode in `OpenDatabase`, checked with backups and encryption
- [ ] `-tags timetzdata` (or `time/tzdata`) in the server build
- [ ] Dockerfile, fly.toml, litestream.yml, entrypoint with restore-on-first-boot
- [ ] Try a restore from the bucket onto a fresh volume
- [ ] Optional: let the server open encrypted databases (password from a Fly secret)
