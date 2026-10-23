# Desktop administration

The separate `cents-admin` executable uses the desktop app's sidebar, styles
and themes. Its first menu item is **Logs**. It connects to the same server
address as the budget desktop app and reads the persistent diagnostic
`logs.db` through an authenticated API.

Build and open it:

```sh
make build-admin
bin/cents-admin
# Or build and open in one step:
make run-admin
```

## Give the admin app access

Rebuild the server to include the administration endpoint, then generate
a separate token:

```sh
make build-server
bin/cents-server token new --name admin --admin --read-only
```

Add the printed YAML entry to `tokens` in your server configuration
(`.deploy/server.yml` for the local Docker setup). It will look like:

```yaml
tokens:
  # Keep your existing bot and desktop entries here too.
  - name: admin
    sha256: THE_GENERATED_SHA256_HASH
    admin: true
    read_only: true
```

Keep the raw token printed by the command for the admin app. The command
prints configuration; it does not edit the file. Existing tokens have no
admin permission unless `admin: true` is explicitly added. `read_only: true`
also prevents this token from changing budget data.

For the combined Docker deployment, rebuild with `make build-docker`, then
recreate your container using the updated image and the same config mounts,
environment variables, port mapping and persistent data volume as before.
Rebuilding the image alone does not update a running container. Reuse your
original `docker run` command when recreating it. `cents-cloud` automatically
connects the administration endpoint to its configured `logs_database`,
normally `/data/logs.db`.

In **cents admin**, enter:

- **Server address:** `http://localhost:8080` for Docker with `-p 8080:8080`.
  For a tunnel or Fly deployment, use its HTTPS root URL, without `/app/`.
- **Admin token:** the raw token, not its SHA-256 hash.

Remembering the connection saves it in `admin.yml` in the OS's cents config
directory, with owner-only file permissions. This is separate from the
budget app's `config.yml`. Uncheck **Remember this connection** for a session
only connection. **Disconnect & forget** clears saved credentials. Remembered
connections reconnect when the admin app starts. Self-signed HTTPS servers
ask you to compare and explicitly trust the certificate fingerprint.

## Inspect logs

Logs appear newest first, with separate **Component**, **Client** and **HTTP
status** columns. Filter by severity, component, client, HTTP status, a literal
substring in messages or JSON attributes, and a time range. HTTP status accepts
an exact code such as `404` or a whole group such as `4xx` or `5xx`. A dash
means that an event has no request status or client. The time controls use your
computer's local timezone; event details also show the exact UTC timestamp.
Click a message to inspect the complete event and its JSON attributes.

**Component** identifies the service that wrote an event: `cloud`, `gateway`,
`server` or `bot`. Gateway events use **Client** to identify `mini_app_api`,
`desktop_api`, `admin_api`, or Mini App assets (`mini_app`). Server events show
the authenticated token's configured name, such as `desktop` or `bot`. Mini
App API requests also produce bot events with `client: mini_app_api`, including
successful requests and authentication failures. Successful static probes are
omitted from the bot's logs.

Historical HTTP statuses and gateway `route` values are read from their stored
attributes, so existing records gain the new columns without migrating the
database. Rebuild the admin app and server image for the new filters and request
logging, then recreate the container with the existing persistent volume.

**Load older** retrieves another 100 events. **Refresh** returns to the newest
matching events. Optional automatic refresh runs every five seconds while
the window is visible. Loading older entries turns it off so you can inspect
history. The app reads logs only; it cannot clear or edit them.

For a standalone `cents-server`, set `logs_database: /path/to/logs.db` in
`server.yml` to enable this endpoint. This must be an existing diagnostic
database created by `cents-cloud`, distinct from `cents.db`. The standalone
server continues to write its own logs to stdout; configuring a log reader
does not add a SQLite log writer to it. An unavailable file returns an error
without creating a replacement. Retention remains controlled by the cloud
configuration.

The read-only endpoint is `GET /api/v1/admin/logs` with bearer authentication.
Query parameters: `limit` (1–500, default 100), `beforeId` (exclusive event ID
cursor), `level`, `component`, `client`, `status` (an exact code or `1xx`–`5xx`),
`search` (up to 200 bytes), `from` and `to`
(inclusive RFC3339 timestamps). Responses contain `entries`, `components`, `clients`,
`hasMore` and `nextBefore`. Ordinary tokens receive 403. The endpoint is
excluded from the Telegram Mini App's API.
