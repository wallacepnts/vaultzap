[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# Docker

With `docker compose`, using `deploy/compose.yml`:

```bash
cd deploy
mkdir -p vaultzap/data vaultzap/inbox
cp vaultzap.env.example vaultzap.env
VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d
```

The `VAULTZAP_UID`/`VAULTZAP_GID` pair is what makes the process write to your binds as
**you** — without it compose falls back to the 1000 default, and on a system where your UID
is anything else the container can't write to the archive. Repeat the prefix on every
`up -d` (it recreates the container). Don't use `UID=$(id -u)`: in bash `UID` is
**readonly**, so the assignment fails, the command runs anyway, and you're silently back to
1000.

Or straight with `docker run`, no compose:

```bash
docker run -d --name vaultzap \
  -p 8927:8927 \
  -v ~/vaultzap/data:/data:rw,z \
  -v /path/to/your/inbox:/inbox:rw,z \
  -e TZ=America/Sao_Paulo -e VAULTZAP_INBOX=/inbox -e VAULTZAP_IMPORTED_DIR=/inbox/.imported \
  --user "$(id -u):$(id -g)" \
  --read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges \
  ghcr.io/wallacepnts/vaultzap:latest
```

**Where your data lives:** both compose mounts are bind mounts with paths relative to
`compose.yml` itself, both under a single folder:

```
vaultzap/
├── data/            <- /data in the container
│   ├── vaultzap.db  <- the database
│   └── media/       <- the attachments' media
└── inbox/           <- /inbox, the watched folder
    └── .imported/     (where an export goes once imported)
```

You decide where the archive lives: **copy `compose.yml` to whatever folder you want** — an
external drive, your Syncthing directory — and `vaultzap/` is created next to it. A bind
rather than a Docker-managed volume because it's your archive: you need to know where the
file is to back it up and to open the `.db` with `sqlite3`, and a `docker compose down -v`
would delete a named volume without asking. Backing up is an `rsync`/`borg` of the whole
`vaultzap/`, with the container stopped.

**Create the folders before the first `up`.** If they don't exist, Docker creates them owned
by root and the process (which runs unprivileged) can't write to them; rootless Podman
doesn't even create them, it aborts.

The inbox mount is `:rw` because the default post-import policy is `move` (the process
removes the file from the inbox after importing); with `VAULTZAP_AFTER_IMPORT=keep`, switch
it to `:ro`. The image is multi-arch (amd64 + arm64) — Docker pulls the right variant on its
own, whether on an x86 server, an Apple Silicon Mac, or a Raspberry Pi.

## Docker rootless

To run without the Docker *daemon* itself as root on the host (not just the process inside
the container), use Docker's official rootless mode:

```bash
curl -fsSL https://get.docker.com/rootless | sh
systemctl --user start docker   # or: export DOCKER_HOST=unix:///run/user/$(id -u)/docker.sock
```

The same `docker compose`/`docker run` commands above work unchanged — Docker's rootless
mode already maps UIDs into its own namespace, so there's no need for `--userns=keep-id`
(that's Podman-specific, below). Official details:
https://docs.docker.com/engine/security/rootless/.
