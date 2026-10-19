# ==============================================================================
# Stage 1 — build
# Compila o binário Go com todos os assets embutidos via go:embed.
# ==============================================================================
FROM golang:1.27-alpine AS builder

# Dependências de build
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Baixar dependências antes de copiar o código-fonte (melhor cache de layer)
COPY go.mod go.sum ./
RUN go mod download

# Copiar o restante do código e compilar
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags "-s -w" \
      -o /out/sdkman-ui \
      ./cmd/server

# ==============================================================================
# Stage 2 — runtime
# Imagem mínima com bash, curl e zip necessários para o SDKMAN funcionar.
# O diretório ~/.sdkman é montado como volume em tempo de execução.
# ==============================================================================
FROM alpine:3.20

# bash é obrigatório (SDKMAN é um conjunto de scripts bash)
# curl e zip são usados pelo instalador e pelo CLI do SDKMAN
RUN apk add --no-cache \
        bash \
        curl \
        zip \
        unzip \
        ca-certificates \
        findutils \
    && addgroup -S appgroup \
    && adduser  -S appuser -G appgroup

# Copiar o binário compilado
COPY --from=builder /out/sdkman-ui /usr/local/bin/sdkman-ui

# Copiar o config padrão (pode ser sobrescrito via volume ou variável)
COPY config.yaml /etc/sdkman-ui/config.yaml

# O SDKMAN precisa de um diretório home para gravar em ~/.sdkman
# Em produção, monte o diretório real do host como volume.
RUN mkdir -p /home/appuser/.sdkman && \
    chown -R appuser:appgroup /home/appuser

USER appuser
ENV HOME=/home/appuser

# Porta padrão — pode ser sobrescrita via config.yaml ou -addr
EXPOSE 8080

# Healthcheck: verifica se o servidor responde na raiz
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -sf http://127.0.0.1:8080/ || exit 1

ENTRYPOINT ["/usr/local/bin/sdkman-ui"]
CMD ["-config", "/etc/sdkman-ui/config.yaml"]
