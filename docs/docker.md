[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Docker

Com `docker compose`, usando `deploy/compose.yml`:

```bash
cd deploy
mkdir -p vaultzap/data vaultzap/inbox
cp vaultzap.env.example vaultzap.env
VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d
```

O par `VAULTZAP_UID`/`VAULTZAP_GID` é o que faz o processo escrever nos seus binds com o
**seu** usuário — sem ele o compose usa o default 1000, e num sistema onde o seu UID é outro
o container não consegue escrever no acervo. Repita o prefixo em todo `up -d` (ele recria o
container). Não use `UID=$(id -u)`: no bash `UID` é **readonly**, o assignment falha, o
comando roda mesmo assim e você volta ao 1000 sem perceber.

Ou direto com `docker run`, sem compose:

```bash
docker run -d --name vaultzap \
  -p 8927:8927 \
  -v ~/vaultzap/data:/data:rw,z \
  -v /caminho/da/sua/inbox:/inbox:rw,z \
  -e TZ=America/Sao_Paulo -e VAULTZAP_INBOX=/inbox -e VAULTZAP_IMPORTED_DIR=/inbox/.imported \
  --user "$(id -u):$(id -g)" \
  --read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges \
  ghcr.io/wallacepnts/vaultzap:latest
```

**Onde ficam seus dados:** os dois mounts do compose são bind mounts com caminho relativo ao
próprio `compose.yml`, os dois debaixo de uma pasta só:

```
vaultzap/
├── data/            <- /data no container
│   ├── vaultzap.db  <- o banco
│   └── media/       <- a mídia dos anexos
└── inbox/           <- /inbox, a pasta observada
    └── .imported/     (pra onde vai o export depois de importado)
```

Quem decide onde o acervo mora é você: **copie o `compose.yml` pra pasta que quiser** — um HD
externo, o diretório do Syncthing — e o `vaultzap/` nasce ao lado dele. Bind, e não volume
gerenciado pelo Docker, porque é o seu acervo: você precisa saber onde o arquivo está pra
fazer backup e pra abrir o `.db` com `sqlite3`, e um `docker compose down -v` apagaria um
volume nomeado sem perguntar nada. Backup é um `rsync`/`borg` do `vaultzap/` inteiro, com o
container parado.

**Crie as pastas antes do primeiro `up`.** Se não existirem, o Docker as cria como root
e o processo (que roda sem privilégio) não consegue escrever; o Podman rootless nem cria,
aborta na hora.

O mount da inbox é `:rw` porque a política padrão pós-import é `move` (o processo tira o
arquivo da inbox depois de importar); com `VAULTZAP_AFTER_IMPORT=keep`, troque pra `:ro`. A
imagem é multi-arch (amd64 + arm64) — o Docker puxa a variante certa sozinho, tanto num
servidor x86 quanto num Mac Apple Silicon ou Raspberry Pi.

## Docker rootless

Pra rodar sem o *daemon* do Docker como root no host (não só o processo dentro do
container), use o modo rootless oficial:

```bash
curl -fsSL https://get.docker.com/rootless | sh
systemctl --user start docker   # ou: export DOCKER_HOST=unix:///run/user/$(id -u)/docker.sock
```

Os mesmos comandos de `docker compose`/`docker run` acima funcionam sem alteração — o
rootless mode do Docker já mapeia os UIDs num namespace próprio, sem precisar de
`--userns=keep-id` (isso é específico do Podman, abaixo). Detalhes oficiais:
https://docs.docker.com/engine/security/rootless/.
