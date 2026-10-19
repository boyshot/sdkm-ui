package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/paulo-rocha/sdkman-ui/internal/api"
	"github.com/paulo-rocha/sdkman-ui/internal/config"
	"github.com/paulo-rocha/sdkman-ui/internal/jobs"
	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
	sdkweb "github.com/paulo-rocha/sdkman-ui/web"
)

func main() {
	// Flags de linha de comando
	cfgPath := flag.String("config", "config.yaml", "caminho para o arquivo de configuração YAML")
	addrFlag := flag.String("addr", "", "sobrescrever endereço de escuta (ex.: 127.0.0.1:9090)")
	flag.Parse()

	// Carregar configuração do arquivo (usa padrões se o arquivo não existir)
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("carregar configuração: %v", err)
	}

	// Flag -addr sobrescreve o endereço definido no arquivo
	addr := cfg.Server.Addr()
	if *addrFlag != "" {
		addr = *addrFlag
	}

	// Dependências
	client := sdkman.NewShellClientWithDir(cfg.SDKMAN.Dir)
	queue := jobs.NewQueueWithSize(client, cfg.Jobs.QueueSize).WithTimeout(cfg.Jobs.Timeout)

	mux := http.NewServeMux()

	// Assets estáticos embutidos
	staticFS, err := fs.Sub(sdkweb.Static, "static")
	if err != nil {
		log.Fatalf("preparar static FS: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Rotas da aplicação
	if err := api.RegisterRoutes(mux, client, queue); err != nil {
		log.Fatalf("registrar rotas: %v", err)
	}

	fmt.Printf("SDKMAN UI iniciando em http://%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("erro no servidor: %v", err)
	}
}
