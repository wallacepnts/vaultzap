[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Configuração

Tudo por variável de ambiente. Nos dois modos de container a configuração vem do **mesmo
arquivo**, no mesmo formato — `deploy/vaultzap.env.example` é o modelo; a cópia real nunca é
versionada:

| onde | arquivo | como recarregar |
|---|---|---|
| quadlet | `~/.config/containers/env/vaultzap.env` (criado por `make quadlet-install`) | `systemctl --user restart vaultzap` |
| compose | `deploy/vaultzap.env` (`cp vaultzap.env.example vaultzap.env`) | `VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d` |

Nenhuma é obrigatória: todas têm default. Os caminhos são os de **dentro** do container —
onde eles caem no host é decidido pelos mounts.

| variável | default | o que faz |
|---|---|---|
| `TZ` | `America/Sao_Paulo` | Fuso usado **só** para decidir "hoje"/"ontem" nos divisores de data. O horário das mensagens vem do export e nunca é convertido. |
| `VAULTZAP_ADDR` | `:8927` | Endereço de escuta. `:9000` muda a porta; `127.0.0.1:8927` limita ao localhost. |
| `VAULTZAP_DB` | `/data/vaultzap.db` | Arquivo SQLite. Criado sozinho na primeira execução. |
| `VAULTZAP_MEDIA_DIR` | `/data/media` | Onde as mídias dos anexos são guardadas, por hash. Criado sozinho. |
| `VAULTZAP_INBOX` | `/inbox` | A pasta observada — o **único** jeito de importar uma conversa. Precisa existir antes. |
| `VAULTZAP_AFTER_IMPORT` | `move` | O que fazer com o arquivo depois de importar. `move`: manda pro `VAULTZAP_IMPORTED_DIR` (nada é apagado). `keep`: não toca em nada, e deixa você montar a inbox `:ro`. `delete`: apaga — nunca quando o import teve avisos. |
| `VAULTZAP_IMPORTED_DIR` | `<inbox>/.imported` | Destino da política `move`. Pode apontar pra fora da inbox (um disco de arquivo morto, por exemplo). Os arquivos são organizados em `AAAA-MM/`. |
| `VAULTZAP_DATE_ORDER` | `DMY` | Desempate de `01/02/2026`. O parser tenta inferir sozinho pelo próprio arquivo; isto só vale quando nem os dados resolvem a ambiguidade. `DMY` ou `MDY`. |
| `VAULTZAP_ME` | vazio | Remetente tratado como "eu" (bolha verde à direita). **Não é um nome à sua escolha**: a comparação é exata contra o texto antes dos dois-pontos nas suas mensagens dentro do export — abra o `.txt` e copie de uma linha sua (`… - Wallace Pontes: Oi` → `VAULTZAP_ME=Wallace Pontes`). Errar não dá erro, só não faz efeito. Costuma ser dispensável: numa conversa 1:1 o app deduz sozinho pelo nome do arquivo, e a escolha da barra "qual destes é você?" é gravada por conversa e tem prioridade sobre esta variável. |
| `VAULTZAP_LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error`. Nunca é registrado conteúdo de mensagem, em nenhum nível. |
| `VAULTZAP_AUTH` | vazio | `off` desliga a autenticação. Qualquer outro valor (ou nenhum) deixa a tela de login, que é o padrão. |
| `VAULTZAP_BASIC_AUTH` | vazio | `usuario:senha` troca a tela de login por Basic Auth. Definir a variável **vazia** é erro no boot — para rodar sem senha, remova a linha e use `VAULTZAP_AUTH=off`. |
| `VAULTZAP_BASIC_AUTH_FILE` | vazio | Caminho de um arquivo contendo `usuario:senha` — é o que permite usar secret do Compose/Podman. Use esta **ou** a de cima, nunca as duas. |

## Como o acesso é protegido

**Tela de login, e é o padrão.** Nada a configurar: no primeiro acesso a um banco novo, o
app mostra uma tela de cadastro onde você escolhe **nome de usuário e senha**. Só o hash da
senha é guardado (PBKDF2-HMAC-SHA256, com salt próprio). Depois disso a tela de cadastro
some, e trocar a senha passa a ser em **Seu perfil → Alterar senha**.

> **Cadastre logo no primeiro boot.** Enquanto ninguém cadastrou, quem alcançar a porta
> primeiro pode fazê-lo. O app avisa isso no log ao subir. Se o serviço não precisa ser
> alcançável pela rede, publique a porta só no localhost — `127.0.0.1:8927:8927` no
> `compose.yml` ou `PublishPort=127.0.0.1:8927:8927` no quadlet.

**Não existe limite de tentativas**, e é decisão consciente: um limitador por IP atrás de um
proxy reverso ou bloquearia todo mundo junto (todos chegam com o IP do proxy) ou seria
contornável trocando um cabeçalho. O que protege é uma senha boa.

**Perdeu a senha?** O binário resolve, sem abrir nada pela rede:

```bash
vaultzap reset-password    # gera uma senha nova, mantém o usuário, encerra as sessões
```

No container: `podman exec vaultzap /vaultzap reset-password`.

**Sem senha**, se algo na frente já protege a porta:

```bash
VAULTZAP_AUTH=off
```

**Basic Auth**, se você prefere o cabeçalho HTTP ao cookie de sessão — é o que já existia, e
tem precedência sobre a tela de login. Duas formas; escolha uma, definir as duas derruba o
app no boot com uma mensagem dizendo isso.

*Simples, senha no arquivo de configuração:*

```bash
# no vaultzap.env
VAULTZAP_BASIC_AUTH=usuario:senha-forte
```

*Por secret, para a senha não ficar em texto puro.* O Compose e o Podman montam secret como
**arquivo**, e o app lê o caminho por `VAULTZAP_BASIC_AUTH_FILE` — a convenção `*_FILE`, a
mesma das imagens de Postgres, MySQL e Nextcloud.

No compose (`deploy/`):

```bash
mkdir -p secrets
printf 'usuario:senha-forte' > secrets/basic-auth.txt
chmod 600 secrets/basic-auth.txt
```

e descomente, no `compose.yml`, as linhas `secrets:`/`environment:` do serviço e o bloco
`secrets:` do fim do arquivo. Elas vêm comentadas de propósito: um secret apontando para um
arquivo que não existe faz o `up` falhar, e quem não quer senha não deveria precisar criar
arquivo nenhum.

No quadlet:

```bash
mkdir -p ~/.config/containers/secrets/vaultzap
printf 'usuario:senha-forte' > ~/.config/containers/secrets/vaultzap/basic-auth.txt
chmod 600 ~/.config/containers/secrets/vaultzap/basic-auth.txt
podman secret create vaultzap-basic-auth ~/.config/containers/secrets/vaultzap/basic-auth.txt
```

e descomente as duas linhas `Secret=`/`Environment=` em `vaultzap.container`. É `type=mount`
e não `type=env` de propósito: como variável de ambiente a senha apareceria num
`podman inspect` do container.

Detalhes que evitam surpresa:

- A quebra de linha no fim do arquivo do secret é ignorada — sem isso ela viraria parte da
  senha e o login falharia sem explicação nenhuma.
- Formato errado (sem os dois-pontos) ou arquivo vazio **derrubam o app no boot**. É
  deliberado: melhor não subir do que subir achando que está protegido.
- `/healthz` fica de fora da autenticação, senão o healthcheck do container falharia sempre e
  o systemd reiniciaria o serviço em laço.
- Basic Auth manda usuário e senha em **base64, não criptografado**. Em `localhost` tudo bem;
  se você expuser na rede, ponha atrás de um proxy com HTTPS.
