[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Desenvolvimento

```bash
make dev    # go run -tags dev ./cmd/vaultzap (templates/CSS recarregam do disco a cada request)
make test   # go test ./...
make lint   # go vet + gofmt -l
make build  # binário estático em ./vaultzap
```

Configuração via variáveis de ambiente `VAULTZAP_*` (endereço, caminho do banco, diretório
de mídia, inbox observada, etc.).

`make image` builda a imagem para a arquitetura do seu host; `make image-multiarch` builda e
publica o manifest amd64 + arm64. **Não é preciso QEMU**: o estágio de build do
`deploy/Dockerfile` roda sempre na plataforma nativa (`--platform=$BUILDPLATFORM`) e
cross-compila via `GOOS`/`GOARCH`, o que funciona porque `CGO_ENABLED=0` e o
`modernc.org/sqlite` é SQLite puro em Go. O estágio final só copia o binário.

Importar um export manualmente (`.txt` solto, `.zip` com mídia, ou uma subpasta com o `.txt`
dentro) — útil para testar sem esperar a próxima varredura da pasta observada:

```bash
go run ./cmd/vaultzap ingest "Conversa do WhatsApp com Fulano.zip"
```

## Rodar sem Docker/Podman

<!-- Seção temporária: remover quando a imagem for publicada em ghcr.io — a partir daí
     "Rodar em container", logo abaixo, já basta. -->

Passo a passo pra rodar o binário direto na sua máquina, sem container — útil enquanto a
imagem não é publicada, ou se você simplesmente não quer usar container.

1. **Instale o Go 1.25+** (https://go.dev/dl/) e confirme com `go version`.

2. **Clone e compile:**

   ```bash
   git clone https://github.com/wallacepnts/vaultzap.git
   cd vaultzap
   make build   # gera ./vaultzap (binário estático, CGO_ENABLED=0)
   ```

3. **Crie as pastas locais.** Os defaults de `VAULTZAP_DB`/`VAULTZAP_MEDIA_DIR`/
   `VAULTZAP_INBOX` (`/data/...`, `/inbox`) pressupõem o layout do container — fora dele,
   aponte pra pastas no seu `$HOME`:

   ```bash
   mkdir -p ~/vaultzap/data ~/vaultzap/media ~/vaultzap/inbox
   ```

4. **Rode apontando as env vars pras pastas do passo 3:**

   ```bash
   VAULTZAP_ADDR=:8927 \
   VAULTZAP_DB="$HOME/vaultzap/data/vaultzap.db" \
   VAULTZAP_MEDIA_DIR="$HOME/vaultzap/media" \
   VAULTZAP_INBOX="$HOME/vaultzap/inbox" \
   ./vaultzap
   ```

   `VAULTZAP_DB` e `VAULTZAP_MEDIA_DIR` são criados sozinhos na primeira execução; a inbox
   precisa existir antes (passo 3). A lista completa de variáveis (`VAULTZAP_AFTER_IMPORT`,
   `VAULTZAP_ME`, `VAULTZAP_BASIC_AUTH`, `VAULTZAP_DATE_ORDER`, etc.) está em
   [Configuração](configuration.md); nenhuma é obrigatória além das três acima.

5. **Abra http://localhost:8927** — sidebar vazia até você soltar um export na inbox.

6. **Solte um `.txt`/`.zip` exportado do WhatsApp em `~/vaultzap/inbox`** e clique em
   "varrer agora" (ícone de pasta na sidebar → `/imports`), ou reinicie o processo — a
   varredura só roda no startup e sob demanda, nunca periodicamente. Com a política
   padrão `VAULTZAP_AFTER_IMPORT=move`, o arquivo importado sai da inbox pra
   `~/vaultzap/inbox/.imported/AAAA-MM/`.

Pra encerrar, `Ctrl+C` — o processo espera a importação/transação corrente terminar antes
de sair (graceful shutdown).
