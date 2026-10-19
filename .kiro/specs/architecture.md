# SDKMAN UI — Especificação de Arquitetura

## 1. Visão Geral

SDKMAN UI é uma aplicação web fullstack escrita em Go que fornece uma interface gráfica para gerenciar instalações do [SDKMAN](https://sdkman.io/). Pode ser executada como **binário único autocontido** ou em container Docker; não requer servidor web externo nem banco de dados.

```
┌─────────────────────────────────────────────────┐
│                   Browser                        │
│  Pico.css + HTMX + SSE (EventSource)            │
└────────────────────┬────────────────────────────┘
                     │ HTTP / SSE
┌────────────────────▼────────────────────────────┐
│              Go HTTP Server                      │
│  net/http ServeMux  ·  endereço configurável     │
│                                                  │
│  ┌─────────────┐  ┌──────────┐  ┌────────────┐ │
│  │  internal/  │  │internal/ │  │ internal/  │ │
│  │    api      │  │  jobs    │  │  sdkman    │ │
│  └──────┬──────┘  └────┬─────┘  └─────┬──────┘ │
│         │              │              │         │
│         └──────────────┴──────────────┘         │
│                   go:embed                       │
│          web/static + web/templates              │
└──────────────────────────────────────────────────┘
                     │
         ┌───────────┴───────────┐
         │  ~/.sdkman/           │
         │  ├── candidates/      │  ← leitura via os.ReadDir
         │  │   └── java/        │
         │  │       ├── current@ │  ← symlink, via os.Readlink
         │  │       └── 21.0.3/  │
         │  └── bin/             │
         │      └── sdkman-init  │  ← exec via bash
         └───────────────────────┘
```

---

## 2. Stack de Tecnologias

| Camada | Tecnologia | Versão | Justificativa |
|---|---|---|---|
| Linguagem | Go | 1.27.2 (`go.mod`) | stdlib robusta, binário único, concorrência nativa |
| HTTP Router | `net/http` ServeMux | stdlib | pattern routing nativo no Go 1.22+, sem dependências |
| Templates | `html/template` | stdlib | escape automático de HTML, sem codegen |
| Frontend | HTMX | 1.9.12 | interatividade sem SPA, requisições declarativas no HTML |
| Estilo | Pico.css | 2.x | CSS semântico, tema claro/escuro, zero classes utilitárias |
| Tempo real | SSE (Server-Sent Events) | — | unidirecional servidor→cliente, ideal para streaming de logs |
| SSE extension | htmx-ext-sse | 2.2.2 | integração HTMX + EventSource |
| Configuração | `gopkg.in/yaml.v3` | 3.0.1 | YAML legível para config file |
| Assets | `go:embed` | stdlib | CSS/JS/templates embutidos no binário |
| Container | Docker multi-stage | Go 1.27 Alpine + Alpine 3.20 | Execução opcional com SDKMAN montado como volume |

---

## 3. Estrutura de Pacotes

```
sdk-ui/
├── cmd/
│   └── server/
│       └── main.go            # Entrypoint: carrega config, monta dependências, inicia HTTP
│
├── internal/
│   ├── config/
│   │   ├── config.go          # Struct Config, Load(), validate(), defaults()
│   │   └── config_test.go
│   │
│   ├── sdkman/
│   │   ├── client.go          # Interface Client + structs Candidate, Version
│   │   ├── shell.go           # ShellClient: leitura FS + exec bash
│   │   ├── mock.go            # MockClient para testes unitários
│   │   ├── validate.go        # ValidateCandidate(), ValidateVersion() (regex)
│   │   └── ansi.go            # stripANSI() — remove escape codes da saída CLI
│   │
│   ├── jobs/
│   │   ├── job.go             # Struct Job: ID, Status, logs, channels
│   │   └── queue.go           # Queue: worker goroutine único, Enqueue, Get
│   │
│   └── api/
│       ├── routes.go          # RegisterRoutes() — monta o mux
│       ├── handlers.go        # Handlers HTTP (index, candidates, versions, install…)
│       ├── sse.go             # Handler SSE /jobs/{id}/stream
│       └── middleware.go      # validateCandidate, validateVersion, chain()
│
├── web/
│   ├── embed.go               # //go:embed static templates
│   ├── static/
│   │   ├── pico.min.css
│   │   ├── htmx.min.js
│   │   └── sse.js
│   └── templates/
│       ├── layout.html        # Base: head, nav, scripts
│       ├── index.html         # Página principal (bloco "content")
│       ├── candidates-list.html  # Partial: tabela de candidates
│       └── versions-list.html   # Partial: tabela de versões + script SSE
│
├── config.yaml                # Configuração padrão (opcional)
├── Dockerfile                 # Build multi-stage e imagem runtime
├── docker-compose.yml         # Execução via Docker Compose
├── Makefile
└── go.mod
```

---

## 4. Fluxo de Dados

### 4.1 Leitura (sem CLI)

```
Browser ──GET /candidates──► handleListCandidates
                                  │
                            ShellClient.ListCandidates()
                                  │
                            os.ReadDir(~/.sdkman/candidates/)
                            os.Readlink(.../current)
                                  │
                            []Candidate ──► HTML partial (HTMX swap)
```

```
Browser ──GET /candidates/{name}/versions──► handleListVersions
                                                  │
                                         ShellClient.ListVersions()
                                                  │
                                    ┌─────────────┴──────────────┐
                                    │                            │
                             API SDKMAN HTTP              os.ReadDir local
                             (versões remotas)            (instaladas)
                                    │                            │
                                    └─────────────┬──────────────┘
                                                  │
                                           []Version (mesclado)
                                                  │
                                        HTML partial (HTMX swap)
```

### 4.2 Escrita com streaming (via CLI)

```
Browser ──POST /candidates/{name}/install──► handleInstall
                                                  │
                                  JSON {"version":"21.0.3-tem"}
                                                  │
                                          Queue.Enqueue()
                                                  │
                                         ← 202 Accepted {"job_id": "42"}
                                                  │
                Browser abre EventSource /jobs/42/stream
                                                  │
                                         handleJobStream (SSE)
                                                  │
                              worker goroutine retira job da fila
                                                  │
                                    ShellClient.Install(ctx, ...)
                                                  │
                              bash -c "source sdkman-init.sh && sdk install java $0"
                                                  │
                              stdout/stderr → bufio.Scanner → stripANSI
                                                  │
                              job.logCh ← linha   │
                                                  │
                              handleJobStream ──► SSE event: log
                                                  │
                              job termina → SSE event: status
                                                  │
                              Browser fecha EventSource, recarrega candidates
```

---

## 5. Componentes em Detalhe

### 5.1 `internal/sdkman` — Integração com SDKMAN

**Interface `Client`:**

```go
type Client interface {
    ListCandidates() ([]Candidate, error)
    ListVersions(candidate string) ([]Version, error)
    Install(ctx context.Context, candidate, version string) (<-chan string, error)
    Uninstall(ctx context.Context, candidate, version string) (<-chan string, error)
    SetDefault(ctx context.Context, candidate, version string) error
}
```

**Integração segura com o shell:**
- Nunca concatena input do usuário em strings shell.
- Usa argumentos posicionais (`$0`, `$1`…) passados ao bash.
- Valida `candidate` e `version` contra `^[a-z0-9._-]+$` antes de qualquer exec.
- Define `SDKMAN_AUTO_ANSWER=true` e `SDKMAN_COLOUR_OUTPUT=false`.
- Remove escape codes ANSI antes de publicar linhas no channel.

### 5.2 `internal/jobs` — Fila de Jobs

- **Serialização**: um único goroutine worker consome o channel; apenas um job executa por vez, evitando conflitos no SDKMAN (que não é thread-safe).
- **ID**: contador atômico (`sync/atomic`), simples e ordenado.
- **Lifecycle**: `pending → running → done | error`.
- **Cancelamento**: ao iniciar a execução, o job recebe um `context.WithTimeout`; o `exec.CommandContext` encerra o processo bash ao cancelar ou expirar o timeout.
- **Distribuição de logs**: channel buffered `logCh` (capacidade 128) por job; o handler SSE consome o channel enquanto a fila acumula as linhas em memória.
- **Persistência e consumo**: jobs e logs existem apenas em memória; o channel de logs é consumível, não um broadcast para múltiplos clientes SSE.

### 5.3 `internal/api` — Camada HTTP

- **Router**: `net/http.ServeMux` com method+path patterns (`GET /candidates/{name}/versions`), disponível nativamente desde Go 1.22.
- **Middleware**: `chain(handler, middlewares...)` aplica `validateCandidate` e `validateVersion` nas rotas que recebem parâmetros de path.
- **HTMX**: os handlers de listagem de candidates e versões detectam `HX-Request: true` e retornam HTML parcial; sem esse header, respondem JSON. Os endpoints de mutação retornam JSON.
- **SSE**: `Content-Type: text/event-stream`, envia os logs já acumulados e depois consome novos; encerra ao fechar `job.logCh`/`job.Done()` ou ao detectar cliente desconectado via `r.Context().Done()`.

### 5.4 `web` — Assets Embutidos

- `//go:embed static` e `//go:embed templates` em `web/embed.go`.
- O binário final inclui CSS, JS e HTML — nenhum arquivo externo necessário.
- `fs.Sub` isola o subdiretório antes de passar ao `http.FileServer`.

---

## 6. API REST

| Método | Rota | Descrição | Retorno |
|---|---|---|---|
| `GET` | `/` | Página principal | HTML |
| `GET` | `/candidates` | Lista candidates instalados | HTML partial / JSON |
| `GET` | `/candidates/{name}/versions` | Versões disponíveis | HTML partial / JSON |
| `POST` | `/candidates/{name}/install` | Inicia instalação; recebe `{"version":"…"}` em JSON | `202 {"job_id":"…"}` |
| `DELETE` | `/candidates/{name}/versions/{version}` | Remove versão | `202 {"job_id":"…"}` |
| `PUT` | `/candidates/{name}/default` | Define versão padrão; recebe `{"version":"…"}` em JSON | `202 {"job_id":"…"}` |
| `GET` | `/jobs/{id}/stream` | SSE stream de log | `text/event-stream` |

**Eventos SSE:**

| Evento | Dados | Descrição |
|---|---|---|
| `log` | linha de texto | Linha de saída do processo |
| `status` | `done` \| `error` | Estado final enviado ao encerrar o stream |
| `error` | mensagem | Detalhe de erro enviado ao encerrar o stream quando disponível |

---

## 7. Configuração

Ordem de precedência (maior sobrescreve menor):

```
1. Valores padrão definidos em internal/config/config.go
2. Arquivo YAML (config.yaml ou caminho informado por -config)
3. Flag de linha de comando -addr (sobrescreve server.host + server.port)
```

Se o arquivo não existir, `config.Load` usa os padrões; campos omitidos no YAML também mantêm seus padrões. O arquivo incluído no projeto define a porta `9000`; o padrão aplicado pelo código quando o campo é omitido é `8080`. O diretório SDKMAN omitido é resolvido como `$HOME/.sdkman`.

Campos disponíveis em `config.yaml`:

```yaml
server:
  host: 127.0.0.1   # IP de escuta
  port: 9000         # porta TCP deste config.yaml

sdkman:
  # dir: /caminho/absoluto/para/.sdkman
  # omitido: $HOME/.sdkman

jobs:
  timeout: 10m       # timeout máximo por job
  queue_size: 64     # capacidade da fila
```

O `Makefile` usa `CONFIG=config.yaml` por padrão em `make run` e permite selecionar outro YAML com `CONFIG=...`; `ADDR=...` é encaminhado como `-addr`. `docker-run` e `docker-up` também montam o arquivo indicado por `CONFIG`. No Docker, o processo escuta em `0.0.0.0:8080`; `PORT` controla a porta publicada no host e não altera a porta interna.

---

## 8. Segurança

| Ameaça | Mitigação |
|---|---|
| Injeção de shell | Argumentos posicionais no bash; nunca interpolação de strings |
| Traversal / path injection | Validação regex `^[a-z0-9._-]+$` em candidate e version |
| Acesso externo não autorizado | Bind local (`127.0.0.1`) nos padrões do código/configuração; Docker força `0.0.0.0` dentro do container e publica a porta do host |
| Execução de comandos arbitrários | Interface `Client` abstrai exec; apenas `install`, `uninstall`, `default` |
| Prompts interativos | `SDKMAN_AUTO_ANSWER=true` elimina confirmações do CLI |

Não há autenticação embutida. Ao usar Docker, a exposição efetiva depende do mapeamento de portas e da rede/firewall do host.

---

## 9. Testabilidade

- **`MockClient`**: implementa `sdkman.Client` com comportamento totalmente configurável via campos públicos e registro de chamadas (`InstallCalls`, `UninstallCalls`, `DefaultCalls`).
- **Handlers**: testáveis via `httptest.NewRecorder` + `httptest.NewRequest` injetando `MockClient`.
- **Queue**: testável com `MockClient` e `WithTimeout(1ms)` para acelerar cenários de timeout.
- **Config**: `Load()` com arquivo real ou arquivo temporário em `/tmp`.

---

## 10. Decisões de Design

| Decisão | Escolha | Alternativa considerada | Motivo |
|---|---|---|---|
| Router | `net/http` ServeMux nativo | `chi` | Evitar dependência externa; Go 1.22 supre a necessidade |
| Templates | `html/template` | `templ` | Sem codegen; mais simples para projeto de escopo reduzido |
| Frontend | HTMX + Pico.css | React + Vite | Menor complexidade; sem build step de JS; HTML server-driven |
| Tempo real | SSE | WebSockets | SSE é suficiente (unidirecional); mais simples de implementar |
| Serialização de jobs | Worker goroutine único | Pool de workers | SDKMAN não é thread-safe; serialização é obrigação, não limitação |
| IDs de jobs | Contador atômico | UUID | Simplicidade; suficiente para uso local single-user |
| Distribuição | Binário único local e imagem Docker | Servidor web externo / banco de dados | Execução local autocontida ou container, sem serviço de aplicação adicional |

---

## 11. Extensibilidade (Backlog)

- **`sdk upgrade`**: adicionar método `Upgrade(ctx, candidate) (<-chan string, error)` ao `Client` e rota `POST /candidates/{name}/upgrade`.
- **`.sdkmanrc`**: leitura do arquivo e aplicação via `sdk use`.
- **Histórico de jobs**: persistir `[]Job` em arquivo JSON ou SQLite para auditoria.
- **Autenticação básica**: middleware de Basic Auth para uso em redes não-localhost.
- **Tema escuro/claro**: alternância via atributo `data-theme` no `<html>`, já suportado pelo Pico.css.
