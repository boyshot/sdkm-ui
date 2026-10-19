# SDKMAN UI — Documentação Técnica

## 1. Visão Geral da Arquitetura

O SDKMAN UI é uma aplicação web fullstack em Go organizada em camadas bem definidas. O binário único embute templates HTML, CSS e JavaScript via `go:embed`, eliminando dependências externas em tempo de execução.

```mermaid
graph TB
    subgraph Browser
        UI[Pico.css + HTMX + EventSource]
    end

    subgraph "Go HTTP Server (127.0.0.1:8080)"
        MW[Middleware\nvalidateCandidate\nvalidateVersion]
        H[Handlers\nhandleIndex\nhandleListCandidates\nhandleListVersions\nhandleInstall\nhandleUninstall\nhandleSetDefault]
        SSE[SSE Handler\nhandleJobStream]
        FS[FileServer\ngo:embed static/]
    end

    subgraph "internal/jobs"
        Q[Queue\nworkCh chan Job]
        W[Worker Goroutine\nunica]
    end

    subgraph "internal/sdkman"
        C[Client interface]
        SC[ShellClient]
        MC[MockClient]
    end

    subgraph "Sistema de Arquivos"
        SDK[~/.sdkman/candidates/]
        INIT[sdkman-init.sh]
    end

    subgraph "Rede Externa"
        API[api.sdkman.io]
    end

    UI -->|HTTP / SSE| MW
    MW --> H
    MW --> SSE
    UI -->|GET /static/| FS
    H --> Q
    H --> C
    SSE --> Q
    Q --> W
    W --> SC
    SC -->|os.ReadDir\nos.Readlink| SDK
    SC -->|exec bash| INIT
    SC -->|HTTP GET| API
    C -.->|implementa| SC
    C -.->|implementa| MC
```

---

## 2. Estrutura de Pacotes

```mermaid
graph LR
    main["cmd/server\nmain.go"]

    subgraph internal
        cfg["config\nConfig\nLoad()"]
        sdk["sdkman\nClient interface\nShellClient\nMockClient\nValidate\nstripANSI"]
        jobs["jobs\nJob\nQueue\nworker()"]
        api["api\nRoutes\nHandlers\nSSE\nMiddleware"]
    end

    subgraph web
        embed["embed.go\ngo:embed"]
        static["static/\npico.min.css\nhtmx.min.js\nsse.js"]
        tmpl["templates/\nlayout.html\nindex.html\ncandidates-list\nversions-list"]
    end

    main --> cfg
    main --> sdk
    main --> jobs
    main --> api
    main --> embed
    api --> sdk
    api --> jobs
    api --> embed
    embed --> static
    embed --> tmpl
```

---

## 3. Modelo de Classes Simplificado

```mermaid
classDiagram
    class Client {
        <<interface>>
        +ListCandidates() []Candidate, error
        +ListVersions(candidate) []Version, error
        +Install(ctx, candidate, version) chan string, error
        +Uninstall(ctx, candidate, version) chan string, error
        +SetDefault(ctx, candidate, version) error
    }

    class ShellClient {
        -httpClient http.Client
        -sdkmanDir string
        +NewShellClient() ShellClient
        +NewShellClientWithDir(dir) ShellClient
        -runSDKCmd(ctx, args) chan string, error
        -installedVersions(candidate) map string
        -resolveCurrentVersion(candidate) string
    }

    class MockClient {
        +CandidatesResult []Candidate
        +VersionsResult map~string~[]Version
        +LogLines []string
        +InstallCalls []CallRecord
        +UninstallCalls []CallRecord
        +DefaultCalls []CallRecord
    }

    class Candidate {
        +Name string
        +CurrentVersion string
    }

    class Version {
        +Version string
        +Installed bool
        +IsCurrent bool
    }

    class Job {
        +ID string
        +Candidate string
        +Version string
        +Action string
        -status Status
        -logs []string
        -logCh chan string
        -done chan struct
        -cancel CancelFunc
        -createdAt time.Time
        +Status() Status
        +Logs() []string
        +LogCh() chan string
        +Done() chan struct
        +Cancel()
    }

    class Queue {
        -jobs map~string~Job
        -workCh chan Job
        -timeout Duration
        -client Client
        +NewQueue(client) Queue
        +NewQueueWithSize(client, size) Queue
        +WithTimeout(d) Queue
        +Enqueue(candidate, version, action) string, error
        +Get(id) Job
        -worker()
        -run(job)
    }

    class Config {
        +Server ServerConfig
        +SDKMAN SDKMANConfig
        +Jobs JobsConfig
        +Load(path) Config, error
    }

    class ServerConfig {
        +Host string
        +Port int
        +Addr() string
    }

    Client <|.. ShellClient : implements
    Client <|.. MockClient  : implements
    ShellClient ..> Candidate : returns
    ShellClient ..> Version   : returns
    Queue       --> Client    : uses
    Queue       --> Job       : manages
```

---

## 4. Modelo de Estados do Job

```mermaid
stateDiagram-v2
    [*] --> pending : Enqueue()

    pending --> running : worker retira da fila

    running --> done  : processo concluído com sucesso
    running --> error : processo falhou ou timeout
    running --> error : ctx.Done() — cancelamento

    done  --> [*]
    error --> [*]

    note right of running
        logCh publica linhas
        SSE handler consome
        e envia ao browser
    end note

    note right of error
        fila cheia também
        resulta em error
        imediatamente
    end note
```

---

## 5. Fluxo de Dados — Leitura

```mermaid
sequenceDiagram
    actor U as Usuário
    participant B as Browser (HTMX)
    participant H as Handler HTTP
    participant SC as ShellClient
    participant FS as Sistema de Arquivos
    participant API as api.sdkman.io

    U->>B: Acessa /
    B->>H: GET /
    H->>SC: ListCandidates()
    SC->>FS: os.ReadDir(~/.sdkman/candidates/)
    FS-->>SC: []DirEntry
    loop Para cada candidate
        SC->>FS: os.Readlink(.../current)
        FS-->>SC: versão atual
    end
    SC-->>H: []Candidate
    H-->>B: HTML (layout + candidates-list)
    B-->>U: Tabela de candidates

    U->>B: Clica "Ver versões" do Java
    B->>H: GET /candidates/java/versions\nHX-Request: true
    H->>SC: ListVersions("java")
    SC->>FS: os.ReadDir(~/.sdkman/candidates/java/)
    FS-->>SC: versões instaladas
    SC->>API: GET /2/candidates/java/linux/versions/all
    API-->>SC: lista CSV de versões
    SC-->>H: []Version (mesclado)
    H-->>B: HTML partial (versions-list)
    B->>B: HTMX swap em #versions-section
    B-->>U: Tabela de versões com badges e botões
```

---

## 6. Fluxo de Dados — Escrita com SSE

```mermaid
sequenceDiagram
    actor U as Usuário
    participant B as Browser
    participant H as Handler HTTP
    participant Q as Queue
    participant W as Worker Goroutine
    participant SC as ShellClient
    participant SH as bash + SDKMAN

    U->>B: Clica "⬇ Instalar"
    B->>H: POST /candidates/java/install\n{"version":"21.0.3-tem"}
    H->>H: validateCandidate("java") ✓
    H->>Q: Enqueue("java","21.0.3-tem","install")
    Q->>Q: newJob() — status: pending
    Q-->>H: jobID = "1"
    H-->>B: 202 {"job_id":"1"}

    B->>H: GET /jobs/1/stream
    H->>H: Content-Type: text/event-stream
    Note over B,H: Conexão SSE aberta

    W->>Q: retira job do workCh
    W->>W: job.status = running
    W->>SC: Install(ctx, "java", "21.0.3-tem")
    SC->>SH: bash -c "source sdkman-init.sh && sdk install java $0"
    SH-->>SC: StdoutPipe aberto

    loop Cada linha de saída
        SH-->>SC: linha raw (com ANSI)
        SC->>SC: stripANSI(linha)
        SC-->>W: logCh <- linha limpa
        W-->>H: job.logCh <- linha
        H-->>B: event: log\ndata: Downloading...
        B->>B: Adiciona linha no #log-panel
    end

    SH-->>SC: processo encerrado (exit 0)
    SC->>SC: close(logCh)
    W->>W: job.status = done
    W->>W: close(job.logCh), close(job.done)
    H-->>B: event: status\ndata: done
    B->>B: Fecha EventSource
    B->>H: GET /candidates (HX-Request: true)
    H-->>B: HTML atualizado
    B-->>U: Lista de candidates reflete instalação
```

---

## 7. Arquitetura de Deploy — Docker

```mermaid
graph TB
    subgraph "Host Linux/macOS"
        SDKMAN["~/.sdkman/\n(instalação real do SDKMAN)"]
        PORT["Porta host :8080"]

        subgraph "Container sdkman-ui (alpine:3.20)"
            BIN["/usr/local/bin/sdkman-ui\n(binário Go estático)"]
            CFG["/etc/sdkman-ui/config.yaml"]
            VOL["/home/appuser/.sdkman\n(volume montado)"]
            BASH["bash + curl + zip\n(runtime SDKMAN)"]
        end

        PORT -->|mapeamento 8080:8080| BIN
        SDKMAN -->|bind mount| VOL
    end

    subgraph "Build Stage (golang:1.27-alpine)"
        SRC["Código-fonte Go\ngo:embed assets"]
        COMP["go build -trimpath\n-ldflags -s -w"]
        SRC --> COMP
    end

    COMP -->|COPY --from=builder| BIN
```

---

## 8. Fluxo de Build Docker Multi-Stage

```mermaid
flowchart LR
    subgraph "Stage 1 — builder"
        A[golang:1.27-alpine] --> B[go mod download]
        B --> C[COPY código-fonte]
        C --> D["go build\nCGO_ENABLED=0\nGOOS=linux\nGOARCH=amd64"]
        D --> E["/out/sdkman-ui\n~15MB binário estático"]
    end

    subgraph "Stage 2 — runtime"
        F[alpine:3.20] --> G["apk add bash curl\nzip unzip ca-certificates"]
        G --> H[adduser appuser]
        H --> I["COPY --from=builder\n/out/sdkman-ui"]
        I --> J["COPY config.yaml\n/etc/sdkman-ui/"]
        J --> K["USER appuser\nEXPOSE 8080\nHEALTHCHECK"]
    end

    E -->|COPY --from=builder| I

    subgraph "Imagem final"
        K --> L["~20MB total\nnão inclui Go toolchain"]
    end
```

---

## 9. Middleware Chain

```mermaid
flowchart LR
    R[HTTP Request] --> VC[validateCandidate\nregex ^a-z0-9._-+$]
    VC -->|inválido| E1[400 Bad Request]
    VC -->|válido| VV[validateVersion\nregex ^a-z0-9._-+$]
    VV -->|inválido| E2[400 Bad Request]
    VV -->|válido| H[Handler de negócio]
    H --> Resp[HTTP Response]
```

---

## 10. Configuração — Precedência

```mermaid
flowchart LR
    D["Defaults\nhardcoded\nhost=127.0.0.1\nport=8080\ntimeout=10m\nqueue=64"]
    Y["config.yaml\n(opcional)"]
    F["Flag -addr\n(opcional)"]
    R["Config final\nusada em runtime"]

    D -->|sobrescrito por| Y
    Y -->|sobrescrito por| F
    F --> R
```

---

## 11. Estrutura de Arquivos — Referência Completa

```
sdk-ui/
├── cmd/
│   └── server/
│       └── main.go               # Entrypoint: monta deps, inicia HTTP
│
├── internal/
│   ├── config/
│   │   ├── config.go             # Config, ServerConfig, SDKMANConfig, JobsConfig
│   │   └── config_test.go        # Testes de Load() e validação
│   │
│   ├── sdkman/
│   │   ├── client.go             # Interface Client, structs Candidate e Version
│   │   ├── shell.go              # ShellClient: FS + exec bash + API HTTP
│   │   ├── mock.go               # MockClient: testes unitários
│   │   ├── validate.go           # ValidateCandidate(), ValidateVersion()
│   │   └── ansi.go               # stripANSI()
│   │
│   ├── jobs/
│   │   ├── job.go                # Job: ID, Status, logCh, done, cancel
│   │   └── queue.go              # Queue: workCh, worker goroutine, Enqueue, Get
│   │
│   └── api/
│       ├── routes.go             # RegisterRoutes()
│       ├── handlers.go           # handleIndex, Candidates, Versions, Install…
│       ├── sse.go                # handleJobStream: SSE streaming
│       └── middleware.go         # validateCandidate, validateVersion, chain()
│
├── web/
│   ├── embed.go                  # //go:embed static templates
│   ├── static/
│   │   ├── pico.min.css          # Pico.css v2 (~83KB)
│   │   ├── htmx.min.js           # HTMX 1.9.12 (~48KB)
│   │   └── sse.js                # htmx-ext-sse 2.2.2 (~9KB)
│   └── templates/
│       ├── layout.html           # Base: head, nav, estilos, scripts
│       ├── index.html            # Página principal (bloco "content")
│       ├── candidates-list.html  # Partial: tabela de candidates
│       └── versions-list.html    # Partial: tabela de versões + SSE script
│
├── doc/
│   ├── business.md               # Documentação de negócio com diagramas
│   └── technical.md              # Este arquivo
│
├── .kiro/specs/
│   └── architecture.md           # Especificação de arquitetura detalhada
│
├── Dockerfile                    # Multi-stage: golang:1.27-alpine + alpine:3.20
├── docker-compose.yml            # Stack Docker Compose
├── .dockerignore
├── config.yaml                   # Configuração padrão
├── Makefile                      # Targets Go e Docker
└── go.mod
```

---

## 12. Decisões Técnicas

| Decisão | Escolha | Razão |
|---|---|---|
| Router HTTP | `net/http.ServeMux` nativo | Go 1.22 suporta method+path patterns sem deps externas |
| Templates | `html/template` stdlib | Escape automático de HTML; sem codegen |
| Frontend | HTMX + Pico.css embutidos | Zero build step; HTML server-driven; sem SPA complexity |
| Streaming | SSE (Server-Sent Events) | Unidirecional, suficiente para log; mais simples que WebSocket |
| Serialização de jobs | Worker goroutine único | SDKMAN não é thread-safe por design |
| Identificador de job | Contador atômico | Simples, ordenado, sem dependências externas (UUID desnecessário) |
| Binário | `go:embed` + build estático | Deploy zero-dependency; funciona offline |
| Container base | `alpine:3.20` | Imagem mínima (~5MB); bash disponível para o SDKMAN |
| Configuração | YAML + flags | Legível para humanos; flags para CI/CD e override pontual |
