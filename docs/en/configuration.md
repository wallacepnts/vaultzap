[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# Configuration

Everything through environment variables. In both container modes the configuration comes
from the **same file**, in the same format — `deploy/vaultzap.env.example` is the template;
the real copy is never versioned:

| where | file | how to reload |
|---|---|---|
| quadlet | `~/.config/containers/env/vaultzap.env` (created by `make quadlet-install`) | `systemctl --user restart vaultzap` |
| compose | `deploy/vaultzap.env` (`cp vaultzap.env.example vaultzap.env`) | `VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d` |

None are required: all have defaults. The paths are the ones **inside** the container — where
they land on the host is decided by the mounts.

| variable | default | what it does |
|---|---|---|
| `TZ` | `America/Sao_Paulo` | Timezone used **only** to decide "today"/"yesterday" in the date dividers. Message times come from the export and are never converted. |
| `VAULTZAP_ADDR` | `:8927` | Listen address. `:9000` changes the port; `127.0.0.1:8927` limits it to localhost. |
| `VAULTZAP_DB` | `/data/vaultzap.db` | The SQLite file. Created automatically on first run. |
| `VAULTZAP_MEDIA_DIR` | `/data/media` | Where attachment media is stored, by hash. Created automatically. |
| `VAULTZAP_INBOX` | `/inbox` | The watched folder — the **only** way to import a conversation. Has to exist beforehand. |
| `VAULTZAP_AFTER_IMPORT` | `move` | What to do with the file after importing. `move`: sends it to `VAULTZAP_IMPORTED_DIR` (nothing is deleted). `keep`: touches nothing, and lets you mount the inbox `:ro`. `delete`: deletes it — never when the import had warnings. |
| `VAULTZAP_IMPORTED_DIR` | `<inbox>/.imported` | Destination for the `move` policy. Can point outside the inbox (a cold-storage drive, say). Files are organised into `YYYY-MM/`. |
| `VAULTZAP_DATE_ORDER` | `DMY` | Tie-breaker for `01/02/2026`. The parser tries to infer it from the file itself; this only applies when the data can't resolve the ambiguity either. `DMY` or `MDY`. |
| `VAULTZAP_ME` | empty | Sender treated as "me" (green bubble on the right). **Not a name of your choosing**: it's an exact match against the text before the colon on your own messages inside the export — open the `.txt` and copy it from one of your lines (`… - Wallace Pontes: Hi` → `VAULTZAP_ME=Wallace Pontes`). Getting it wrong isn't an error, it just has no effect. Usually unnecessary: in a 1:1 chat the app works it out from the filename, and the choice made in the "which of these is you?" bar is stored per conversation and takes precedence over this variable. |
| `VAULTZAP_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. Message content is never logged, at any level. |
| `VAULTZAP_AUTH` | empty | `off` turns authentication off. Anything else (or nothing) keeps the login screen, which is the default. |
| `VAULTZAP_BASIC_AUTH` | empty | `user:password` swaps the login screen for Basic Auth. Setting the variable **empty** is an error at boot — to run without a password, remove the line and use `VAULTZAP_AUTH=off`. |
| `VAULTZAP_BASIC_AUTH_FILE` | empty | Path to a file containing `user:password` — this is what lets you use a Compose/Podman secret. Use this **or** the one above, never both. |

## How access is protected

**A login screen, and it is the default.** Nothing to configure: on the first visit to a
new database the app shows a setup screen where you pick a **username and password**. Only
the password's hash is stored (PBKDF2-HMAC-SHA256, with its own salt). After that the setup
screen is gone, and changing the password happens under **Your profile → Change password**.

> **Set it up right after the first boot.** Until someone does, whoever reaches the port
> first can. The app says so in the log at startup. If the service does not need to be
> reachable from the network, publish the port on localhost only —
> `127.0.0.1:8927:8927` in `compose.yml`, or `PublishPort=127.0.0.1:8927:8927` in the
> quadlet.

There is **no attempt limiter**, and that is deliberate: a per-IP limiter behind a reverse
proxy would either lock everyone out together (every request carries the proxy's IP) or be
sidestepped by changing a header. What protects the archive is a good password.

**Lost the password?** The binary handles it, without opening anything over the network:

```bash
vaultzap reset-password    # generates a new password, keeps the username, ends sessions
```

In the container: `podman exec vaultzap /vaultzap reset-password`.

**No password**, if something in front already guards the port:

```bash
VAULTZAP_AUTH=off
```

**Basic Auth**, if you would rather have the HTTP header than a session cookie — it is what
already existed, and it takes precedence over the login screen. Two ways; pick one, setting
both stops the app at boot with a message saying so.

*Simple, password in the configuration file:*

```bash
# in vaultzap.env
VAULTZAP_BASIC_AUTH=user:strong-password
```

*Through a secret, so the password isn't in plain text.* Compose and Podman both mount
secrets as **files**, and the app reads the path from `VAULTZAP_BASIC_AUTH_FILE` — the
`*_FILE` convention, the same one the Postgres, MySQL and Nextcloud images use.

On compose (in `deploy/`):

```bash
mkdir -p secrets
printf 'user:strong-password' > secrets/basic-auth.txt
chmod 600 secrets/basic-auth.txt
```

then uncomment, in `compose.yml`, the service's `secrets:`/`environment:` lines and the
`secrets:` block at the end of the file. They ship commented on purpose: a secret pointing at
a file that doesn't exist makes `up` fail, and someone who doesn't want a password shouldn't
have to create any file.

On the quadlet:

```bash
mkdir -p ~/.config/containers/secrets/vaultzap
printf 'user:strong-password' > ~/.config/containers/secrets/vaultzap/basic-auth.txt
chmod 600 ~/.config/containers/secrets/vaultzap/basic-auth.txt
podman secret create vaultzap-basic-auth ~/.config/containers/secrets/vaultzap/basic-auth.txt
```

then uncomment the two `Secret=`/`Environment=` lines in `vaultzap.container`. It's
`type=mount` and not `type=env` on purpose: as an environment variable the password would
show up in a `podman inspect` of the container.

Details that save you a surprise:

- The trailing newline in the secret file is ignored — without that it would become part of
  the password and the login would fail with no explanation at all.
- A wrong format (no colon) or an empty file **stops the app at boot**. That's deliberate:
  better not to start than to start thinking it's protected.
- `/healthz` is left out of authentication, otherwise the container's healthcheck would
  always fail and systemd would restart the service in a loop.
- Basic Auth sends user and password **base64-encoded, not encrypted**. On `localhost` that's
  fine; if you expose it on a network, put it behind an HTTPS proxy.
