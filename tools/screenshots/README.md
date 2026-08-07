# Screenshots

Regenerates every image under `docs/img/pt/` and `docs/img/en/`.
Not part of the build; only `.work/` is left out of git.

```bash
python3 tools/screenshots/build.py        # both languages, 9 images each
python3 tools/screenshots/build.py pt     # one language
```

Needs `chromium`, `pngquant`, Pillow and the Go toolchain — it runs the app from
source on a random port, imports fictional conversations, pins two messages so the
pinned strip has real state, photographs each screen and quantizes the PNGs.

Everything it writes lives in `.work/` (throwaway inbox and database) except the
images themselves, which land in `docs/img/`.

Notes worth keeping in mind when changing it:

- The gallery, calendar, search, profile and merge panels are htmx fragments with no
  navigable URL. Each page is assembled: `GET /` for the real layout, `GET` the
  fragment with `HX-Request: true`, splice, add `<base href>`.
- `VAULTZAP_ME` is deliberately left unset, so the 1:1 chats get an inferred owner and
  the group doesn't — which is what makes the "which of these is you?" bar appear.
- Fictional data only (§9.7). Message text is generated combinatorially so the search
  panel doesn't show the same sentence fifteen times.
