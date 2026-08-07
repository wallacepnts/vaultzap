[← Voltar ao README](../README.md) · [Configuração](configuration.md) · [Desenvolvimento](development.md) · [Docker](docker.md) · [Podman](podman.md) · [Quadlet](quadlet.md) · [Uso](usage.md) · [Problemas](troubleshooting.md)

# Quando algo dá errado

Sintoma → causa → o que fazer. Antes de tudo, dois lugares que respondem quase todas as
perguntas:

- **a página de importações** (ícone de pasta na sidebar): mostra cada arquivo visto, o que
  entrou, o que foi ignorado, os avisos e a mensagem de erro;
- **o log do container**: `docker logs -f vaultzap`, `podman logs -f vaultzap` ou
  `journalctl --user -u vaultzap -f` no quadlet.

Conteúdo de mensagem nunca é registrado no log, em nenhum nível — só contagens e caminhos.

## Soltei o arquivo na inbox e não aconteceu nada

Quase sempre é isto: **a varredura não é periódica.** Ela roda no startup do app e quando
você clica em **"varrer agora"** na página de importações. Clique nele e espere alguns
segundos — a demora é a segunda leitura da pasta, que existe pra não tocar num arquivo ainda
sendo copiado.

Se mesmo assim não aparecer:

- **O arquivo ainda está sendo sincronizado.** Nomes terminados em `.part`, `.crdownload`,
  `.filepart`, `.tmp`, `.!qB` e arquivos começando com ponto são ignorados de propósito —
  são cópias pela metade. Espere o Syncthing/navegador terminar.
- **Ele já foi importado antes.** Com a política padrão `move`, o arquivo importado sai da
  inbox pra `.imported/AAAA-MM/`. Se ele está lá, a conversa já está no acervo.
- **Você apagou essa conversa e recolocou o mesmo arquivo.** Apagar marca o arquivo como
  ignorado justamente pra ele não voltar sozinho na varredura seguinte. Um `touch` no arquivo
  (ou copiá-lo de novo) muda tamanho/data e ele volta a ser importado.
- **A página de importações marca erro.** Abra o item: a mensagem diz o motivo.

## O `.zip` foi recusado

O que o descompactador recusa é **expansão desproporcional**, não tamanho: um zip bomb
descomprime centenas de vezes o próprio tamanho, enquanto um export do WhatsApp é quase todo
mídia — que já vem comprimida e sai perto de 1:1. Um export de 7 GB com 14 mil arquivos entra
normalmente.

Se a recusa mencionar **zip bomb**, o arquivo declara expandir mais de 100× em alguma entrada
ou no total. Entrada com `..` ou caminho absoluto no nome também é recusada — é tentativa de
escrever fora da pasta de destino.

**Espaço em disco:** durante o import, o zip é descompactado dentro do `VAULTZAP_MEDIA_DIR`
antes de os anexos irem para o lugar definitivo, então conte com **cerca de 2× o tamanho do
export** livre enquanto ele roda. O temporário é apagado no fim.

## As fotos não aparecem, só um espaço reservado

O `.txt` cita o anexo mas o arquivo não veio junto: é o que acontece quando você exporta
**"sem mídia"**. Exporte de novo escolhendo **"com mídia"** e solte o `.zip` na inbox — as
mensagens não duplicam, e as fotos entram nas que já estavam lá.

Se as duas versões viraram conversas separadas (nomes de arquivo diferentes), use **Mesclar
conversa**: a mensagem que sobrevive herda o anexo da outra.

## Todas as bolhas ficam à esquerda

O app não sabe qual remetente é você. Numa conversa 1:1 ele deduz pelo nome do arquivo; em
grupo, ou quando o arquivo foi renomeado, não dá — e ele não chuta.

Responda a barra **"qual destes é você?"** no topo da conversa (ou refaça a escolha em
**Dados do contato**). Para um padrão global, `VAULTZAP_ME` — mas o valor tem que ser
**exatamente** o texto antes dos dois-pontos nas suas linhas do export, não um nome à sua
escolha.

## A mesma conversa apareceu duas vezes

O app casa export com conversa pelo **nome do arquivo**. Contato que mudou de número,
conversa renomeada aqui, ou export vindo do chatvault (pasta com UUID) geram nome diferente
e, portanto, uma segunda conversa.

- Já aconteceu: **Mesclar conversa** no menu da que deve ficar. As mensagens repetidas não
  duplicam.
- Prevenir: solte o arquivo na inbox com o app aberto e use **Atualizar conversa** no menu da
  conversa certa, em vez de deixar a varredura decidir.

## As datas estão trocadas (dia virou mês)

`01/02/2026` é ambíguo, e o parser resolve pelo próprio arquivo — quando algum dia passa de
12, ou quando uma das leituras produz datas fora de ordem. Se o export inteiro for ambíguo,
vale `VAULTZAP_DATE_ORDER` (`DMY` ou `MDY`). Reimporte depois de mudar.

## Aparecem avisos na importação

Aviso não é falha: a linha que o parser não entendeu vira uma mensagem de sistema com o texto
bruto, e o resto do arquivo entra normalmente. O painel do item mostra **a linha** do export
em cada aviso — dá pra corrigir o `.txt` e usar **"Reimportar…"** ali mesmo.

E **"ignoradas" não são mensagens perdidas**: são as que já estavam no acervo.

## O container não sobe / reinicia em laço

**`unable to open database file (14)`** — no Podman/quadlet, é o `keep-id` incompleto. Tem
que ser `--userns=keep-id:uid=65532,gid=65532` (ou `UserNS=keep-id:uid=65532,gid=65532` na
unit): `keep-id` sozinho mapeia o seu UID pro mesmo número dentro do container, mas a imagem
roda como UID 65532, que não alcança os seus arquivos. Detalhes em [Podman](podman.md).

**`permission denied` mesmo com o UID certo e a pasta sendo sua** — falta o rótulo SELinux
`:z` nos dois mounts (`-v ~/vaultzap/data:/data:rw,z`). Acontece em qualquer distro com
SELinux ativo, não só Fedora/RHEL. Em sistema sem SELinux o rótulo é ignorado, então pode
deixar sempre.

**O processo escreve com o usuário errado (compose)** — o prefixo é
`VAULTZAP_UID=$(id -u) VAULTZAP_GID=$(id -g) docker compose up -d`. Não use `UID=$(id -u)`:
no bash `UID` é readonly, o assignment falha, o comando roda mesmo assim e o compose cai no
default 1000.

**`address already in use`** — outra coisa está na porta 8927. Troque com
`VAULTZAP_ADDR=:9000` (e a publicação de porta correspondente).

## Erro de "mover" a cada varredura

A política padrão é `move`, e a inbox está montada `:ro`. Ou monte `:rw` (é o que o
`compose.yml` e a unit já trazem), ou troque para `VAULTZAP_AFTER_IMPORT=keep`, que não toca
em arquivo nenhum.

## O app não sobe depois de ligar a senha

Todos estes derrubam no boot de propósito — melhor não subir do que subir achando que está
protegido:

- `defina VAULTZAP_BASIC_AUTH ou VAULTZAP_BASIC_AUTH_FILE, não as duas`;
- `VAULTZAP_BASIC_AUTH inválido: formato esperado usuario:senha` — faltam os dois-pontos;
- `VAULTZAP_BASIC_AUTH_FILE: arquivo ... está vazio`.

Se o app sobe mas **o login não passa**, confira se não há espaço extra no arquivo do secret.
A quebra de linha final é ignorada de propósito; espaço no meio, não.

## O healthcheck falha sempre

`/healthz` fica fora da autenticação justamente pra isso, então senha não é a causa. Num
quadlet, confira que `HealthCmd` está em array JSON (`["/vaultzap", "healthcheck"]`): em
texto solto o Podman tenta rodar via `/bin/sh`, que **não existe** na imagem distroless — a
checagem sai com erro sempre e sem nenhuma pista no log.

## Nada disso?

Rode com `VAULTZAP_LOG_LEVEL=debug` e olhe o log durante uma varredura. Se o problema for de
leitura do export, o caminho mais rápido é reproduzir fora do container:

```bash
go run ./cmd/vaultzap ingest "Conversa do WhatsApp com Fulano.zip"
```

que importa de forma síncrona e imprime o relatório do parser na tela.
