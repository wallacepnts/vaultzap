[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Quadlet

Pra quem já roda tudo com Podman/systemd e quer o container gerenciado como uma unit, sem
`podman run` manual — integra direto com `systemctl --user`:

```bash
make quadlet-install
systemctl --user daemon-reload
systemctl --user start vaultzap
loginctl enable-linger $USER   # opcional: sobe sem sessão aberta
```

O `make quadlet-install` monta esta árvore no seu `~/.config/containers`:

```
~/.config/containers/
├── systemd/
│   └── vaultzap.container      <- a unit (um arquivo só, então fica solto aqui)
├── env/
│   └── vaultzap.env            <- a configuração; editar aqui basta um restart
├── volumes/
│   └── vaultzap/
│       ├── data/               <- /data: vaultzap.db + media/
│       └── inbox/              <- /inbox: onde você solta os exports
└── secrets/
    └── vaultzap/               <- só se você ligar o Basic Auth (abaixo)
```

A regra de subpasta em `systemd/` é a do próprio Podman: um único arquivo quadlet fica solto;
a partir de dois (por exemplo se um dia entrar um `.network`) eles vão para uma subpasta
`systemd/vaultzap/`.

**Configuração fica no `.env`, não na unit.** Trocar `TZ` ou `VAULTZAP_AFTER_IMPORT` é editar
`~/.config/containers/env/vaultzap.env` e `systemctl --user restart vaultzap` — sem
`daemon-reload`, que só é preciso quando o `.container` muda. O `quadlet-install` copia o
`.env` com `cp -n`, então nunca sobrescreve o que você já ajustou.

**A inbox aponta para `volumes/vaultzap/inbox` por padrão.** Se a sua for uma pasta
sincronizada (Syncthing, Samba, KDE Connect), troque o caminho do `Volume=...:/inbox` na unit.

**Basic Auth via secret do Podman**, para a senha não ficar em texto puro no `.env`:

```bash
mkdir -p ~/.config/containers/secrets/vaultzap
printf 'usuario:senha-forte' > ~/.config/containers/secrets/vaultzap/basic-auth.txt
podman secret create vaultzap-basic-auth ~/.config/containers/secrets/vaultzap/basic-auth.txt
```

e descomente a linha `Secret=` em `vaultzap.container`. Os arquivos em `secrets/` são a
fonte do segredo — não versione essa pasta.

Antes do primeiro `start`, ajuste `Image=` para o registry publicado. A unit já vem com
`UserNS=keep-id:uid=65532,gid=65532` e o rótulo `:z` (mesmos motivos do Podman acima) e a mesma regra `:rw`/`:ro`.

## Backup

Todo o acervo está em `~/.config/containers/volumes/vaultzap/` — apesar do nome da pasta,
são bind mounts, arquivos comuns seus. `data/` tem o `vaultzap.db` e a mídia; `inbox/` tem o
que ainda não foi importado e o `.imported/` com os exports originais.

```bash
systemctl --user stop vaultzap
rsync -a ~/.config/containers/volumes/vaultzap/ /destino/do/backup/
systemctl --user start vaultzap
```

Restaurar é o contrário: pare o serviço, ponha a pasta de volta, inicie. O `.db` é SQLite
comum — `sqlite3 ~/.config/containers/volumes/vaultzap/data/vaultzap.db` lê o acervo sem o
VaultZap no meio. Se você apontou o `Volume=...:/inbox` para outro lugar (uma pasta do
Syncthing), inclua esse caminho no backup também.
