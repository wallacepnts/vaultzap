[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# Quadlet

For anyone already running everything on Podman/systemd and who wants the container managed
as a unit, without a manual `podman run` — integrates directly with `systemctl --user`:

```bash
make quadlet-install
systemctl --user daemon-reload
systemctl --user start vaultzap
loginctl enable-linger $USER   # optional: starts without an open session
```

`make quadlet-install` lays out this tree under your `~/.config/containers`:

```
~/.config/containers/
├── systemd/
│   └── vaultzap.container      <- the unit (a single file, so it sits loose here)
├── env/
│   └── vaultzap.env            <- the configuration; editing it only needs a restart
├── volumes/
│   └── vaultzap/
│       ├── data/               <- /data: vaultzap.db + media/
│       └── inbox/              <- /inbox: where you drop the exports
└── secrets/
    └── vaultzap/               <- only if you turn Basic Auth on (below)
```

The subfolder rule under `systemd/` is Podman's own: a single quadlet file sits loose; from
two up (say a `.network` shows up one day) they move into a `systemd/vaultzap/` subfolder.

**Configuration lives in the `.env`, not in the unit.** Changing `TZ` or
`VAULTZAP_AFTER_IMPORT` means editing `~/.config/containers/env/vaultzap.env` and
`systemctl --user restart vaultzap` — no `daemon-reload`, which is only needed when the
`.container` itself changes. `quadlet-install` copies the `.env` with `cp -n`, so it never
overwrites what you already tuned.

**The inbox points at `volumes/vaultzap/inbox` by default.** If yours is a synced folder
(Syncthing, Samba, KDE Connect), change the `Volume=...:/inbox` path in the unit.

**Basic Auth through a Podman secret**, so the password isn't sitting in plain text in the
`.env`:

```bash
mkdir -p ~/.config/containers/secrets/vaultzap
printf 'user:strong-password' > ~/.config/containers/secrets/vaultzap/basic-auth.txt
podman secret create vaultzap-basic-auth ~/.config/containers/secrets/vaultzap/basic-auth.txt
```

then uncomment the `Secret=` line in `vaultzap.container`. The files under `secrets/` are the
secret's source — don't version that folder.

Before the first `start`, set `Image=` to the published registry. The unit already ships with
`UserNS=keep-id:uid=65532,gid=65532` and the `:z` label (same reasons as Podman above) and the same `:rw`/`:ro`
rule.

## Backup

The whole archive lives in `~/.config/containers/volumes/vaultzap/` — despite the folder's
name, these are bind mounts, ordinary files of yours. `data/` holds `vaultzap.db` and the
media; `inbox/` holds what hasn't been imported yet plus `.imported/` with the original
exports.

```bash
systemctl --user stop vaultzap
rsync -a ~/.config/containers/volumes/vaultzap/ /path/to/backup/
systemctl --user start vaultzap
```

Restoring is the reverse: stop the service, put the folder back, start it. The `.db` is
ordinary SQLite — `sqlite3 ~/.config/containers/volumes/vaultzap/data/vaultzap.db` reads the
archive with no VaultZap in the way. If you pointed `Volume=...:/inbox` somewhere else (a
Syncthing folder), include that path in the backup too.
