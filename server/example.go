package server

// ExampleConfig is a starting point for the config file.
const ExampleConfig = `# cents server config.

# Where to listen. 127.0.0.1 is this machine only; use 0.0.0.0:8740 (or one
# of the machine's addresses) to let others in.
listen: 127.0.0.1:8740

# The SQLite database, made with the desktop app.
database: /path/to/cents.db

# Who may connect: addresses and networks (CIDR). Everyone else is turned
# away before anything else happens. Empty means this machine only.
allow:
  - 127.0.0.1
  - ::1
  # - 192.168.1.0/24

# Only these proxy peers may supply the original client in Fly-Client-IP.
# Keep empty when clients connect directly. cents-cloud sets this to loopback.
trusted_proxies: []

# HTTPS. Needed to listen beyond this machine, unless insecure_http is set.
tls:
  cert: ""
  key: ""

# Plain HTTP to other machines: only for testing on a network you trust,
# as tokens and data travel unencrypted.
insecure_http: false

# Clients allowed in. Make one with: cents-server token new --name NAME
# and paste what it prints here; the token itself goes to the client. Add
# read_only: true to a client that should only read.
tokens: []

# Request and event logs: format text or json; level debug, info, warn or
# error.
log:
  format: text
  level: info
`
