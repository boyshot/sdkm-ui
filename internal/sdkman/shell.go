package sdkman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// sdkmanRoot é o diretório base do SDKMAN.
	sdkmanRoot = "~/.sdkman"

	// sdkmanInit é o script de inicialização do SDKMAN.
	sdkmanInit = "~/.sdkman/bin/sdkman-init.sh"

	// apiVersionsURL é o endpoint da API pública do SDKMAN para versões.
	apiVersionsURL = "https://api.sdkman.io/2/candidates/%s/linux/versions/all"

	// httpTimeout é o timeout para chamadas à API remota do SDKMAN.
	httpTimeout = 30 * time.Second
)

// ShellClient é a implementação real de Client que integra com o SDKMAN
// instalado localmente via sistema de arquivos e CLI bash.
type ShellClient struct {
	httpClient *http.Client
	// sdkmanDir permite sobrescrever o diretório base em testes.
	sdkmanDir string
}

// NewShellClient cria um ShellClient com configurações padrão.
func NewShellClient() *ShellClient {
	home, _ := os.UserHomeDir()
	return &ShellClient{
		httpClient: &http.Client{Timeout: httpTimeout},
		sdkmanDir:  filepath.Join(home, ".sdkman"),
	}
}

// NewShellClientWithDir cria um ShellClient usando o diretório SDKMAN informado.
// Se dir for vazio, usa o padrão ~/.sdkman.
func NewShellClientWithDir(dir string) *ShellClient {
	if dir == "" {
		return NewShellClient()
	}
	return &ShellClient{
		httpClient: &http.Client{Timeout: httpTimeout},
		sdkmanDir:  dir,
	}
}

// candidatesDir retorna o caminho para o diretório de candidates.
func (c *ShellClient) candidatesDir() string {
	return filepath.Join(c.sdkmanDir, "candidates")
}

// initScript retorna o caminho para o script de inicialização do SDKMAN.
func (c *ShellClient) initScript() string {
	return filepath.Join(c.sdkmanDir, "bin", "sdkman-init.sh")
}

// ListCandidates lê os candidates instalados em ~/.sdkman/candidates/.
// A versão atual de cada candidate é resolvida pelo symlink "current".
func (c *ShellClient) ListCandidates() ([]Candidate, error) {
	entries, err := os.ReadDir(c.candidatesDir())
	if err != nil {
		return nil, fmt.Errorf("ler diretório de candidates: %w", err)
	}

	var candidates []Candidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		current, _ := c.resolveCurrentVersion(name)
		candidates = append(candidates, Candidate{
			Name:           name,
			CurrentVersion: current,
		})
	}
	return candidates, nil
}

// resolveCurrentVersion resolve o symlink "current" de um candidate
// e retorna o nome da versão ativa, ou string vazia se não existir.
func (c *ShellClient) resolveCurrentVersion(candidate string) (string, error) {
	link := filepath.Join(c.candidatesDir(), candidate, "current")
	target, err := os.Readlink(link)
	if err != nil {
		return "", nil // sem versão corrente definida
	}
	return filepath.Base(target), nil
}

// installedVersions retorna um set das versões instaladas localmente.
func (c *ShellClient) installedVersions(candidate string) (map[string]struct{}, error) {
	dir := filepath.Join(c.candidatesDir(), candidate)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ler versões de %s: %w", candidate, err)
	}
	installed := make(map[string]struct{})
	for _, e := range entries {
		if e.IsDir() && e.Name() != "current" {
			installed[e.Name()] = struct{}{}
		}
	}
	return installed, nil
}

// apiVersionsResponse representa a resposta da API do SDKMAN para versões.
type apiVersionsResponse struct {
	Versions []string `json:"versions"`
}

// ListVersions busca versões disponíveis na API do SDKMAN e mescla com
// as versões instaladas localmente.
func (c *ShellClient) ListVersions(candidate string) ([]Version, error) {
	if err := ValidateCandidate(candidate); err != nil {
		return nil, err
	}

	// Versões instaladas localmente
	installed, err := c.installedVersions(candidate)
	if err != nil {
		// Se o candidate não existe localmente, continua com set vazio
		installed = make(map[string]struct{})
	}

	// Versão corrente
	current, _ := c.resolveCurrentVersion(candidate)

	// Buscar versões disponíveis na API remota
	url := fmt.Sprintf(apiVersionsURL, candidate)
	resp, err := c.httpClient.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("buscar versões de %s: %w", candidate, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API SDKMAN retornou %d para %s", resp.StatusCode, candidate)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ler resposta da API: %w", err)
	}

	// A API retorna uma string com versões separadas por vírgula ou JSON.
	// Tentar JSON primeiro; se falhar, interpretar como CSV.
	var versions []Version
	var parsed apiVersionsResponse
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Versions) > 0 {
		for _, v := range parsed.Versions {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			_, isInstalled := installed[v]
			versions = append(versions, Version{
				Version:   v,
				Installed: isInstalled,
				IsCurrent: v == current,
			})
		}
	} else {
		// Formato CSV: "21.0.3-tem,21.0.2-tem,..."
		for _, v := range strings.Split(string(body), ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			_, isInstalled := installed[v]
			versions = append(versions, Version{
				Version:   v,
				Installed: isInstalled,
				IsCurrent: v == current,
			})
		}
	}

	return versions, nil
}

// runSDKCmd executa um comando SDK via bash, capturando a saída linha a linha
// e publicando cada linha no channel retornado. O channel é fechado ao término.
// Nunca concatena entrada do usuário na string do shell — usa argumentos posicionais.
func (c *ShellClient) runSDKCmd(ctx context.Context, args ...string) (<-chan string, error) {
	// Montar o comando bash com source do sdkman-init.sh
	// Os argumentos são passados como $0, $1, ... para evitar injeção de shell.
	scriptArgs := make([]string, 0, len(args)+2)
	scriptArgs = append(scriptArgs, "-c",
		fmt.Sprintf(`source "%s" && sdk %s`,
			c.initScript(),
			buildSDKArgRefs(len(args)),
		),
	)
	scriptArgs = append(scriptArgs, args...)

	cmd := exec.CommandContext(ctx, "bash", scriptArgs...)
	cmd.Env = append(os.Environ(),
		"SDKMAN_AUTO_ANSWER=true",
		"SDKMAN_COLOUR_OUTPUT=false",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("abrir stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("abrir stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("iniciar comando sdk: %w", err)
	}

	ch := make(chan string, 64)

	go func() {
		defer close(ch)
		defer cmd.Wait() //nolint:errcheck

		// Ler stdout e stderr em paralelo
		merged := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(merged)
		for scanner.Scan() {
			line := stripANSI(scanner.Text())
			if line == "" {
				continue
			}
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// buildSDKArgRefs constrói a string de referências posicionais "$0 $1 $2..."
// para n argumentos, usada na construção segura do comando bash.
func buildSDKArgRefs(n int) string {
	refs := make([]string, n)
	for i := range refs {
		refs[i] = fmt.Sprintf("$%d", i)
	}
	return strings.Join(refs, " ")
}

// Install inicia a instalação de uma versão via `sdk install`.
func (c *ShellClient) Install(ctx context.Context, candidate, version string) (<-chan string, error) {
	if err := ValidateCandidate(candidate); err != nil {
		return nil, err
	}
	if err := ValidateVersion(version); err != nil {
		return nil, err
	}
	return c.runSDKCmd(ctx, "install", candidate, version)
}

// Uninstall remove uma versão instalada via `sdk uninstall`.
func (c *ShellClient) Uninstall(ctx context.Context, candidate, version string) (<-chan string, error) {
	if err := ValidateCandidate(candidate); err != nil {
		return nil, err
	}
	if err := ValidateVersion(version); err != nil {
		return nil, err
	}
	return c.runSDKCmd(ctx, "uninstall", candidate, version)
}

// SetDefault define a versão padrão de um candidate via `sdk default`.
func (c *ShellClient) SetDefault(ctx context.Context, candidate, version string) error {
	if err := ValidateCandidate(candidate); err != nil {
		return err
	}
	if err := ValidateVersion(version); err != nil {
		return err
	}
	ch, err := c.runSDKCmd(ctx, "default", candidate, version)
	if err != nil {
		return err
	}
	// Consumir o channel para aguardar o término do comando
	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	// Verificar se houve erro na saída
	for _, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "error") || strings.Contains(lower, "not installed") {
			return fmt.Errorf("sdk default falhou: %s", strings.Join(lines, "; "))
		}
	}
	return nil
}
