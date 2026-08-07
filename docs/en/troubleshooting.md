[← Back to the README](../../README.en.md) · [Configuration](configuration.md) · [Development](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Usage](usage.md) · [Troubleshooting](troubleshooting.md)

# When something goes wrong

Symptom → cause → what to do. First, two places that answer almost every question:

- **the imports page** (folder icon in the sidebar): every file seen, what went in, what was
  skipped, the warnings and the error message;
- **the container log**: `docker logs -f vaultzap`, `podman logs -f vaultzap`, or
  `journalctl --user -u vaultzap -f` on the quadlet.

Message content is never logged, at any level — only counts and paths.

## I dropped the file in the inbox and nothing happened

Almost always this: **scanning isn't periodic.** It runs when the app starts and when you
click **"scan now"** on the imports page. Click it and wait a few seconds — the delay is the
second read of the folder, which exists so it never touches a file that's still being copied.

If it still doesn't show up:

- **The file is still syncing.** Names ending in `.part`, `.crdownload`, `.filepart`,
  `.tmp`, `.!qB` and files starting with a dot are ignored on purpose — they're half-copies.
  Wait for Syncthing/your browser to finish.
- **It was already imported.** With the default `move` policy, an imported file leaves the
  inbox for `.imported/YYYY-MM/`. If it's there, the conversation is already in the archive.
- **You deleted that conversation and put the same file back.** Deleting marks the file as
  ignored precisely so it doesn't come back on its own at the next scan. A `touch` on the
  file (or copying it again) changes size/date and it gets imported again.
- **The imports page shows an error.** Open the item: the message says why.

## The `.zip` was rejected

What the extractor rejects is **disproportionate expansion**, not size: a zip bomb blows up
to hundreds of times its own size, while a WhatsApp export is nearly all media — already
compressed, coming out close to 1:1. A 7 GB export with 14,000 files goes in fine.

If the rejection mentions **zip bomb**, the file declares an expansion of more than 100x on
some entry or in total. An entry whose name has `..` or an absolute path is rejected too —
that's an attempt to write outside the destination folder.

**Disk space:** during the import the zip is unpacked inside `VAULTZAP_MEDIA_DIR` before the
attachments move to their final place, so keep **about 2x the size of the export** free while
it runs. The temporary copy is removed at the end.

## Photos don't show up, just a placeholder

The `.txt` names the attachment but the file didn't come with it: that's what happens when
you export **"without media"**. Export again choosing **"with media"** and drop the `.zip`
into the inbox — messages don't duplicate, and the photos attach to the ones already there.

If the two versions became separate conversations (different filenames), use **Merge
conversation**: the surviving message inherits the other one's attachment.

## Every bubble is on the left

The app doesn't know which sender is you. In a 1:1 conversation it works it out from the
filename; in a group, or when the file was renamed, it can't — and it won't guess.

Answer the **"which of these is you?"** bar at the top of the conversation (or redo the
choice under **Contact info**). For a global default, `VAULTZAP_ME` — but the value has to be
**exactly** the text before the colon on your own lines in the export, not a name of your
choosing.

## The same conversation showed up twice

The app matches an export to a conversation by **filename**. A contact who changed numbers, a
conversation renamed here, or an export from chatvault (a UUID-named folder) all produce a
different name and therefore a second conversation.

- Already happened: **Merge conversation** from the menu of the one that should stay.
  Repeated messages don't duplicate.
- Preventing it: drop the file into the inbox with the app open and use **Update
  conversation** from the right conversation's menu, instead of letting the scan decide.

## The dates are swapped (day became month)

`01/02/2026` is ambiguous, and the parser resolves it from the file itself — when some day
goes past 12, or when one of the readings produces out-of-order dates. If the whole export is
ambiguous, `VAULTZAP_DATE_ORDER` (`DMY` or `MDY`) decides. Re-import after changing it.

## The import shows warnings

A warning isn't a failure: the line the parser didn't understand becomes a system message
holding the raw text, and the rest of the file goes in normally. The item's panel shows **the
line** of the export behind each warning — you can fix the `.txt` and use **"Reimport…"**
right there.

And **"skipped" doesn't mean lost**: those are the ones already in the archive.

## The container won't start / restarts in a loop

**`unable to open database file (14)`** — on Podman/quadlet, that's an incomplete `keep-id`.
It has to be `--userns=keep-id:uid=65532,gid=65532` (or `UserNS=keep-id:uid=65532,gid=65532`
in the unit): `keep-id` on its own maps your UID to the same number inside the container, but
the image runs as UID 65532, which can't reach your files. Details in [Podman](podman.md).

**`permission denied` even with the right UID and the folder being yours** — the SELinux `:z`
label is missing from both mounts (`-v ~/vaultzap/data:/data:rw,z`). It happens on any distro
with SELinux enabled, not just Fedora/RHEL. On a system without SELinux the label is ignored,
so you can leave it on always.

**The process writes as the wrong user (compose)** — the prefix is
`VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d`. Don't use `UID=$(id -u)`:
in bash `UID` is readonly, the assignment fails, the command runs anyway, and compose falls
back to the 1000 default.

**`address already in use`** — something else is on port 8927. Change it with
`VAULTZAP_ADDR=:9000` (and the matching port publish).

## A "move" error on every scan

The default policy is `move`, and the inbox is mounted `:ro`. Either mount it `:rw` (which is
what `compose.yml` and the unit already do), or switch to `VAULTZAP_AFTER_IMPORT=keep`, which
touches no files at all.

## The app won't start after turning the password on

All of these stop it at boot on purpose — better not to start than to start thinking it's
protected:

- `defina VAULTZAP_BASIC_AUTH ou VAULTZAP_BASIC_AUTH_FILE, não as duas` (set one, not both);
- `VAULTZAP_BASIC_AUTH inválido: formato esperado usuario:senha` — the colon is missing;
- `VAULTZAP_BASIC_AUTH_FILE: arquivo ... está vazio` — the file is empty.

If the app starts but **the login won't go through**, check for a stray space in the secret
file. The trailing newline is ignored on purpose; a space in the middle isn't.

## The healthcheck always fails

`/healthz` is left out of authentication precisely for this, so a password isn't the cause. On
a quadlet, check that `HealthCmd` is a JSON array (`["/vaultzap", "healthcheck"]`): as loose
text, Podman tries to run it through `/bin/sh`, which **doesn't exist** in the distroless
image — the check fails every time with no hint in the log.

## None of the above?

Run with `VAULTZAP_LOG_LEVEL=debug` and watch the log during a scan. If the problem is in
reading the export, the fastest path is reproducing it outside the container:

```bash
go run ./cmd/vaultzap ingest "WhatsApp Chat with Jane.zip"
```

which imports synchronously and prints the parser's report to the screen.
