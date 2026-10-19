package sdkman

import "context"

// MockClient é uma implementação de Client para uso em testes.
// Todos os comportamentos são configuráveis via campos públicos.
type MockClient struct {
	// CandidatesResult é retornado por ListCandidates.
	CandidatesResult []Candidate
	CandidatesErr    error

	// VersionsResult é retornado por ListVersions (keyed por candidate).
	VersionsResult map[string][]Version
	VersionsErr    error

	// LogLines são as linhas publicadas nos channels de Install/Uninstall.
	LogLines []string

	// InstallErr é retornado por Install se não-nil.
	InstallErr error

	// UninstallErr é retornado por Uninstall se não-nil.
	UninstallErr error

	// SetDefaultErr é retornado por SetDefault se não-nil.
	SetDefaultErr error

	// Chamadas registradas para verificação nos testes
	InstallCalls   []CallRecord
	UninstallCalls []CallRecord
	DefaultCalls   []CallRecord
}

// CallRecord registra os argumentos de uma chamada de método.
type CallRecord struct {
	Candidate string
	Version   string
}

func (m *MockClient) ListCandidates() ([]Candidate, error) {
	return m.CandidatesResult, m.CandidatesErr
}

func (m *MockClient) ListVersions(candidate string) ([]Version, error) {
	if m.VersionsErr != nil {
		return nil, m.VersionsErr
	}
	if m.VersionsResult != nil {
		return m.VersionsResult[candidate], nil
	}
	return nil, nil
}

func (m *MockClient) Install(ctx context.Context, candidate, version string) (<-chan string, error) {
	m.InstallCalls = append(m.InstallCalls, CallRecord{candidate, version})
	if m.InstallErr != nil {
		return nil, m.InstallErr
	}
	return publishLines(ctx, m.LogLines), nil
}

func (m *MockClient) Uninstall(ctx context.Context, candidate, version string) (<-chan string, error) {
	m.UninstallCalls = append(m.UninstallCalls, CallRecord{candidate, version})
	if m.UninstallErr != nil {
		return nil, m.UninstallErr
	}
	return publishLines(ctx, m.LogLines), nil
}

func (m *MockClient) SetDefault(ctx context.Context, candidate, version string) error {
	m.DefaultCalls = append(m.DefaultCalls, CallRecord{candidate, version})
	return m.SetDefaultErr
}

// publishLines cria um channel que publica as linhas fornecidas e fecha.
func publishLines(ctx context.Context, lines []string) <-chan string {
	ch := make(chan string, len(lines))
	go func() {
		defer close(ch)
		for _, l := range lines {
			select {
			case ch <- l:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}
