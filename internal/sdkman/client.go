package sdkman

import "context"

// Candidate representa um candidate do SDKMAN (ex.: java, maven, gradle).
type Candidate struct {
	Name           string // nome do candidate (ex.: "java")
	CurrentVersion string // versão ativa (symlink current), vazia se nenhuma
}

// Version representa uma versão de um candidate.
type Version struct {
	Version   string // identificador da versão (ex.: "21.0.3-tem")
	Installed bool   // true se já está instalada localmente
	IsCurrent bool   // true se é a versão ativa (current)
}

// Client define as operações disponíveis sobre o SDKMAN.
// Possui implementação real (ShellClient) e mock (MockClient) para testes.
type Client interface {
	// ListCandidates retorna todos os candidates instalados localmente.
	ListCandidates() ([]Candidate, error)

	// ListVersions retorna versões disponíveis (API remota) mescladas com
	// as versões instaladas localmente para o candidate informado.
	ListVersions(candidate string) ([]Version, error)

	// Install inicia a instalação de uma versão e retorna um channel onde
	// cada linha do log de progresso é publicada. O channel é fechado ao término.
	Install(ctx context.Context, candidate, version string) (<-chan string, error)

	// Uninstall remove uma versão instalada e publica o log no channel retornado.
	Uninstall(ctx context.Context, candidate, version string) (<-chan string, error)

	// SetDefault define a versão padrão de um candidate.
	SetDefault(ctx context.Context, candidate, version string) error
}
