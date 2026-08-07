[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# Development

```bash
make dev    # go run -tags dev ./cmd/vaultzap (templates/CSS reload from disk on every request)
make test   # go test ./...
make lint   # go vet + gofmt -l
make build  # static binary at ./vaultzap
```

Configuration via `VAULTZAP_*` environment variables (address, database path, media
directory, watched inbox, etc.).

`make image` builds the image for your host's architecture; `make image-multiarch` builds and
publishes the amd64 + arm64 manifest. **No QEMU is needed**: the build stage in
`deploy/Dockerfile` always runs on the native platform (`--platform=$BUILDPLATFORM`) and
cross-compiles through `GOOS`/`GOARCH`, which works because `CGO_ENABLED=0` and
`modernc.org/sqlite` is pure-Go SQLite. The final stage only copies the binary.

Importing an export manually (a loose `.txt`, a `.zip` with media, or a subfolder containing
the `.txt`) — useful for testing without waiting for the next watched-folder scan:

```bash
go run ./cmd/vaultzap ingest "WhatsApp Chat with John.zip"
```

## Running without Docker/Podman

Step by step to run the binary directly on your machine, without a container. The main path
is the published image (see [Docker](docker.md), [Podman](podman.md) and
[Quadlet](quadlet.md)); this one is for whoever would rather not use a container.

1. **Install Go 1.25+** (https://go.dev/dl/) and confirm with `go version`.

2. **Clone and build:**

   ```bash
   git clone https://github.com/wallacepnts/vaultzap.git
   cd vaultzap
   make build   # produces ./vaultzap (static binary, CGO_ENABLED=0)
   ```

3. **Create local folders.** The defaults for `VAULTZAP_DB`/`VAULTZAP_MEDIA_DIR`/
   `VAULTZAP_INBOX` (`/data/...`, `/inbox`) assume the container layout — outside of it,
   point them at folders under your `$HOME`:

   ```bash
   mkdir -p ~/vaultzap/data ~/vaultzap/media ~/vaultzap/inbox
   ```

4. **Run it pointing the env vars at the folders from step 3:**

   ```bash
   VAULTZAP_ADDR=:8927 \
   VAULTZAP_DB="$HOME/vaultzap/data/vaultzap.db" \
   VAULTZAP_MEDIA_DIR="$HOME/vaultzap/media" \
   VAULTZAP_INBOX="$HOME/vaultzap/inbox" \
   ./vaultzap
   ```

   `VAULTZAP_DB` and `VAULTZAP_MEDIA_DIR` are created automatically on first run; the inbox
   needs to exist beforehand (step 3). The full variable list (`VAULTZAP_AFTER_IMPORT`,
   `VAULTZAP_ME`, `VAULTZAP_BASIC_AUTH`, `VAULTZAP_DATE_ORDER`, etc.) is in
   [Configuration](configuration.md); none are required beyond the three above.

5. **Open http://localhost:8927** — the sidebar is empty until you drop an export into the
   inbox.

6. **Drop an exported `.txt`/`.zip` from WhatsApp into `~/vaultzap/inbox`** and click "scan
   now" (folder icon in the sidebar → `/imports`), or restart the process — scanning only
   runs on startup and on demand, never periodically. With the default `move` policy
   (`VAULTZAP_AFTER_IMPORT=move`), the imported file leaves the inbox for
   `~/vaultzap/inbox/.imported/YYYY-MM/`.

To stop it, `Ctrl+C` — the process waits for the current import/transaction to finish before
exiting (graceful shutdown).
