# SDKMAN UI — Documentação de Negócio

## 1. Visão do Produto

O **SDKMAN UI** resolve um problema cotidiano de desenvolvedores que trabalham com múltiplas versões de ferramentas JVM: o gerenciamento via linha de comando do SDKMAN é eficiente, mas exige memorização de comandos e não oferece visibilidade rápida do estado atual das instalações.

O produto oferece uma interface web local, leve e segura que traduz as operações do SDKMAN em ações visuais simples — sem abandonar a robustez e o controle que o SDKMAN já oferece.

---

## 2. Personas

### Dev Backend — "Marco"
- Trabalha com múltiplos projetos Java simultâneos, cada um exigindo uma versão diferente do JDK.
- Troca de versão frequentemente e precisa saber rapidamente qual está ativa.
- Não lembra os identificadores exatos das versões disponíveis.

### Tech Lead — "Ana"
- Padroniza o ambiente de desenvolvimento da equipe.
- Precisa instalar e definir versões padrão em múltiplas máquinas.
- Valoriza rastreabilidade: quer ver o log de cada operação realizada.

### Dev Júnior — "Lucas"
- Recém-chegado ao ecossistema JVM.
- Prefere interfaces visuais a comandos de terminal.
- Comete erros ao digitar versões; se beneficia da validação automática.

---

## 3. Casos de Uso

```mermaid
flowchart LR
    Dev([Desenvolvedor])

    UC1(Visualizar candidates instalados)
    UC2(Ver versões disponíveis)
    UC3(Instalar uma versão)
    UC4(Remover uma versão)
    UC5(Definir versão padrão)
    UC6(Acompanhar progresso em tempo real)

    Dev --> UC1
    Dev --> UC2
    Dev --> UC3
    Dev --> UC4
    Dev --> UC5
    UC3 --> UC6
    UC4 --> UC6
    UC5 --> UC6
```

---

## 4. Glossário de Negócio

| Termo | Definição |
|---|---|
| **Candidate** | Ferramenta gerenciada pelo SDKMAN (Java, Maven, Gradle, Kotlin, Scala…) |
| **Versão** | Identificador único de uma release de um candidate (ex.: `21.0.3-tem`) |
| **Versão atual (current)** | Versão ativa no momento — apontada pelo symlink `current` |
| **Versão instalada** | Versão baixada e presente localmente em `~/.sdkman/candidates/` |
| **Versão disponível** | Versão listada na API do SDKMAN mas ainda não baixada |
| **Job** | Operação assíncrona de escrita (install, uninstall, default) com log de progresso |
| **SSE** | Server-Sent Events — mecanismo de streaming do log do job para o browser |

---

## 5. Regras de Negócio

| ID | Regra |
|---|---|
| RN01 | Apenas uma operação de escrita pode estar em execução por vez |
| RN02 | Uma versão não pode ser removida se for a versão atual (current) |
| RN03 | Candidatos e versões devem seguir o padrão `^[a-z0-9._-]+$` |
| RN04 | O servidor só aceita conexões de `127.0.0.1` por padrão |
| RN05 | Operações de escrita têm timeout máximo configurável (padrão: 10 minutos) |
| RN06 | A fila de jobs tem capacidade máxima configurável (padrão: 64) |

---

## 6. Fluxos de Negócio

### 6.1 Fluxo principal — Visualizar estado do ambiente

```mermaid
flowchart TD
    A([Usuário acessa a aplicação]) --> B[Carrega página inicial]
    B --> C[Servidor lê ~/.sdkman/candidates/]
    C --> D{Há candidates instalados?}
    D -- Sim --> E[Exibe tabela com candidate e versão atual]
    D -- Não --> F[Exibe mensagem: nenhum candidate encontrado]
    E --> G([Usuário visualiza o ambiente])
```

---

### 6.2 Fluxo — Explorar versões de um candidate

```mermaid
flowchart TD
    A([Usuário clica em 'Ver versões']) --> B[Requisição GET /candidates/name/versions]
    B --> C[Servidor consulta versões locais em ~/.sdkman]
    C --> D[Servidor consulta API remota do SDKMAN]
    D --> E[Mescla versões locais e remotas]
    E --> F[Retorna lista com status de cada versão]
    F --> G{Status da versão}
    G -- current  --> H[Badge verde 'current']
    G -- instalada --> I[Badge azul 'instalada' + ações: Padrão, Remover]
    G -- disponível --> J[Sem badge + ação: Instalar]
```

---

### 6.3 Fluxo — Instalar uma versão

```mermaid
sequenceDiagram
    actor U as Usuário
    participant B as Browser
    participant S as Servidor HTTP
    participant Q as Fila de Jobs
    participant CLI as SDKMAN CLI (bash)

    U->>B: Clica em "⬇ Instalar"
    B->>S: POST /candidates/{name}/install {"version":"..."}
    S->>S: Valida candidate e version (regex)
    S->>Q: Enqueue(candidate, version, "install")
    Q-->>S: job_id = "42"
    S-->>B: 202 Accepted {"job_id":"42"}

    B->>S: GET /jobs/42/stream (SSE)
    S-->>B: Conexão SSE aberta

    Q->>CLI: bash -c "source sdkman-init.sh && sdk install java $0" 21.0.3-tem
    loop Cada linha de saída
        CLI-->>Q: linha de log
        Q-->>S: logCh <- linha
        S-->>B: event: log\ndata: Downloading...
    end

    CLI-->>Q: processo encerrado
    Q-->>S: job.status = done
    S-->>B: event: status\ndata: done
    B->>B: Fecha EventSource
    B->>S: GET /candidates (recarregar lista)
    S-->>B: HTML atualizado
    B-->>U: Interface reflete nova instalação
```

---

### 6.4 Fluxo — Definir versão padrão

```mermaid
flowchart TD
    A([Usuário clica em '★ Padrão']) --> B[PUT /candidates/name/default\nbody: version]
    B --> C{Versão está instalada?}
    C -- Não --> D[400 Bad Request]
    C -- Sim --> E[Enfileira job 'default']
    E --> F[Worker executa: sdk default candidate version]
    F --> G[SDKMAN atualiza symlink current]
    G --> H[Job finaliza com status 'done']
    H --> I[Browser recarrega lista de candidates]
    I --> J([Badge 'current' atualizado na interface])
```

---

### 6.5 Fluxo — Remover uma versão

```mermaid
flowchart TD
    A([Usuário clica em '✕ Remover']) --> B{Versão é a current?}
    B -- Sim --> C[Botão desabilitado — ação bloqueada na UI]
    B -- Não --> D[Exibe diálogo de confirmação]
    D --> E{Usuário confirma?}
    E -- Não --> F([Operação cancelada])
    E -- Sim --> G[DELETE /candidates/name/versions/version]
    G --> H[Enfileira job 'uninstall']
    H --> I[Worker executa: sdk uninstall candidate version]
    I --> J[Job finaliza]
    J --> K([Interface atualizada sem a versão removida])
```

---

## 7. Jornada do Usuário

```mermaid
journey
    title Desenvolvedor gerenciando versões do Java
    section Verificar ambiente
      Acessa o SDKMAN UI no browser: 5: Dev
      Vê candidates instalados e versão ativa: 4: Dev
    section Explorar versões
      Clica em "Ver versões" do Java: 5: Dev
      Visualiza versões instaladas e disponíveis: 4: Dev
    section Instalar nova versão
      Clica em "⬇ Instalar" na versão desejada: 5: Dev
      Acompanha o log de progresso em tempo real: 4: Dev
      Vê o status "done" ao concluir: 5: Dev
    section Definir padrão
      Clica em "★ Padrão" na versão instalada: 5: Dev
      Badge "current" atualiza automaticamente: 5: Dev
    section Limpeza
      Remove versão antiga desnecessária: 4: Dev
      Confirma no diálogo e acompanha remoção: 4: Dev
```

---

## 8. Escopo MVP vs. Futuro

### MVP (implementado)

| Funcionalidade | Status |
|---|---|
| Listar candidates instalados | ✅ |
| Listar versões disponíveis | ✅ |
| Instalar versão com log em tempo real | ✅ |
| Remover versão instalada | ✅ |
| Definir versão padrão | ✅ |
| Interface web responsiva | ✅ |
| Configuração via arquivo YAML | ✅ |
| Execução via Docker | ✅ |

### Backlog futuro

| Funcionalidade | Prioridade |
|---|---|
| Atualizar candidates desatualizados (`sdk upgrade`) | Alta |
| Suporte a `.sdkmanrc` por projeto | Alta |
| Histórico de operações realizadas | Média |
| `sdk use` (versão temporária na sessão) | Média |
| Notificação de novas versões disponíveis | Baixa |
| Autenticação para uso em rede local | Baixa |
