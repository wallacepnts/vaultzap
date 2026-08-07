[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Podman

Rootless por padrão, sem daemon. Com `podman compose`, usando o mesmo `deploy/compose.yml`:

```bash
cd deploy
mkdir -p vaultzap/data vaultzap/inbox
cp vaultzap.env.example vaultzap.env
VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) podman compose up -d
```

O prefixo `VAULTZAP_UID`/`VAULTZAP_GID` vale igual aqui, inclusive o motivo de não se chamar
`UID` — está explicado em [Docker](docker.md).

Ou direto com `podman run`:

```bash
podman run -d --name vaultzap \
  -p 8927:8927 \
  -v ~/vaultzap/data:/data:rw,z \
  -v /caminho/da/sua/inbox:/inbox:rw,z \
  -e TZ=America/Sao_Paulo -e VAULTZAP_INBOX=/inbox -e VAULTZAP_IMPORTED_DIR=/inbox/.imported \
  --userns=keep-id:uid=65532,gid=65532 \
  --read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges \
  ghcr.io/wallacepnts/vaultzap:latest
```

`--userns=keep-id:uid=65532,gid=65532` faz o UID dentro do container coincidir com o seu no
host. O `:uid=`/`:gid=` não é enfeite: `keep-id` sozinho mapeia o seu UID pro **mesmo número**
dentro do container (1000 → 1000), mas a imagem distroless roda como `nonroot`, UID 65532 —
que cai num subuid sem acesso nenhum às suas pastas, e o container morre no boot com
`unable to open database file`. A forma completa mapeia o seu UID justamente pro 65532.
O `:z` no mount
da inbox e no de `/data` é o rótulo SELinux compartilhado: sem ele dá `permission denied`
mesmo com o UID certo e a pasta sendo sua — confirmado numa openSUSE com SELinux ativo, não
só em Fedora/RHEL. Em sistema sem SELinux o rótulo é ignorado, então pode deixar sempre.
Mesma regra `:rw`/`:ro` do Docker acima. O `deploy/compose.yml` já traz o `:z` nos dois
mounts.

## Onde ficam seus dados

Igual ao Docker: os dois mounts do compose são bind mounts relativos ao próprio
`compose.yml`, debaixo de uma pasta só.

```
vaultzap/
├── data/            <- /data no container
│   ├── vaultzap.db  <- o banco
│   └── media/       <- a mídia dos anexos
└── inbox/           <- /inbox, a pasta observada
    └── .imported/     (pra onde vai o export depois de importado)
```

Com `podman run`, é o que você escolheu em `-v` (nos exemplos acima, `~/vaultzap/data` e a
sua pasta de inbox).

**Backup é copiar `vaultzap/` inteiro**, com o container parado — `rsync`, `borg`, o que você
já usar. Restaurar é pôr a pasta de volta e subir de novo. O `.db` é SQLite comum: `sqlite3
vaultzap.db` abre e lê o acervo sem o VaultZap no meio. Guarde também os exports originais
em `.imported/` — são a cópia que não depende deste programa.
