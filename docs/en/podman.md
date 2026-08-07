[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# Podman

Rootless by default, no daemon. With `podman compose`, using the same `deploy/compose.yml`:

```bash
cd deploy
mkdir -p vaultzap/data vaultzap/inbox
cp vaultzap.env.example vaultzap.env
VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) podman compose up -d
```

The `VAULTZAP_UID`/`VAULTZAP_GID` prefix applies here too, including why it isn't called
`UID` — explained in [Docker](docker.md).

Or straight with `podman run`:

```bash
podman run -d --name vaultzap \
  -p 8927:8927 \
  -v ~/vaultzap/data:/data:rw,z \
  -v /path/to/your/inbox:/inbox:rw,z \
  -e TZ=America/Sao_Paulo -e VAULTZAP_INBOX=/inbox -e VAULTZAP_IMPORTED_DIR=/inbox/.imported \
  --userns=keep-id:uid=65532,gid=65532 \
  --read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges \
  ghcr.io/wallacepnts/vaultzap:latest
```

`--userns=keep-id:uid=65532,gid=65532` makes the UID inside the container match yours on the
host. The `:uid=`/`:gid=` part is not decoration: `keep-id` on its own maps your UID to the
**same number** inside the container (1000 → 1000), but the distroless image runs as
`nonroot`, UID 65532 — which lands on a subuid with no access at all to your folders, and the
container dies at boot with `unable to open database file`. The full form maps your UID onto
65532 instead. The `:z` on the inbox and `/data` mounts is the shared SELinux label: without it you get `permission
denied` even with the right UID and the folder being yours — confirmed on an openSUSE with
SELinux enabled, not just on Fedora/RHEL. On a system without SELinux the label is ignored,
so you can always leave it on. Same `:rw`/`:ro` rule as Docker above. `deploy/compose.yml`
already carries `:z` on both mounts.

## Where your data lives

Same as Docker: both compose mounts are bind mounts relative to `compose.yml` itself, under a
single folder.

```
vaultzap/
├── data/            <- /data in the container
│   ├── vaultzap.db  <- the database
│   └── media/       <- attachment media
└── inbox/           <- /inbox, the watched folder
    └── .imported/     (where exports go once imported)
```

With `podman run`, it's whatever you passed to `-v` (in the examples above, `~/vaultzap/data`
and your own inbox folder).

**Backup is copying the whole `vaultzap/`**, with the container stopped — `rsync`, `borg`,
whatever you already use. Restoring is putting the folder back and starting it again. The
`.db` is ordinary SQLite: `sqlite3 vaultzap.db` opens and reads the archive with no VaultZap
in the way. Keep the original exports in `.imported/` too — they're the copy that doesn't
depend on this program.
