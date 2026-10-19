// Package config carrega a configuração da aplicação a partir de um arquivo
// YAML e/ou variáveis de ambiente, com valores padrão sensatos.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config contém todas as opções configuráveis do SDKMAN UI.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	SDKMAN  SDKMANConfig  `yaml:"sdkman"`
	Jobs    JobsConfig    `yaml:"jobs"`
}

// ServerConfig agrupa as configurações do servidor HTTP.
type ServerConfig struct {
	// Host é o endereço IP de escuta. Padrão: 127.0.0.1
	Host string `yaml:"host"`
	// Port é a porta TCP. Padrão: 8080
	Port int `yaml:"port"`
}

// Addr retorna a string "host:port" pronta para uso em http.ListenAndServe.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// SDKMANConfig agrupa as configurações relacionadas ao SDKMAN local.
type SDKMANConfig struct {
	// Dir é o diretório base do SDKMAN. Padrão: ~/.sdkman
	Dir string `yaml:"dir"`
}

// JobsConfig agrupa as configurações da fila de jobs.
type JobsConfig struct {
	// Timeout é o timeout máximo para execução de um job. Padrão: 10m
	Timeout time.Duration `yaml:"timeout"`
	// QueueSize é a capacidade máxima da fila. Padrão: 64
	QueueSize int `yaml:"queue_size"`
}

// defaults retorna a configuração com todos os valores padrão aplicados.
func defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		SDKMAN: SDKMANConfig{
			Dir: home + "/.sdkman",
		},
		Jobs: JobsConfig{
			Timeout:   10 * time.Minute,
			QueueSize: 64,
		},
	}
}

// Load carrega a configuração a partir do arquivo YAML no caminho informado.
// Se o arquivo não existir, retorna os valores padrão sem erro.
// Campos ausentes no arquivo mantêm o valor padrão.
func Load(path string) (Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Arquivo opcional — usar padrões
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("ler arquivo de configuração %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsear configuração YAML %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("configuração inválida: %w", err)
	}

	return cfg, nil
}

// validate verifica a consistência dos valores carregados.
func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d fora do intervalo válido (1–65535)", c.Server.Port)
	}
	if c.Server.Host == "" {
		return fmt.Errorf("server.host não pode ser vazio")
	}
	if c.Jobs.Timeout <= 0 {
		return fmt.Errorf("jobs.timeout deve ser positivo")
	}
	if c.Jobs.QueueSize < 1 {
		return fmt.Errorf("jobs.queue_size deve ser >= 1")
	}
	return nil
}
