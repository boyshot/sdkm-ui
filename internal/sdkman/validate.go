package sdkman

import (
	"fmt"
	"regexp"
)

// reName valida nomes de candidate e versão.
// Permite letras minúsculas, dígitos, ponto, hífen e underscore.
var reName = regexp.MustCompile(`^[a-z0-9._-]+$`)

// ValidateCandidate retorna erro se o nome do candidate for inválido.
func ValidateCandidate(candidate string) error {
	if candidate == "" || !reName.MatchString(candidate) {
		return fmt.Errorf("candidate inválido: %q", candidate)
	}
	return nil
}

// ValidateVersion retorna erro se o identificador de versão for inválido.
func ValidateVersion(version string) error {
	if version == "" || !reName.MatchString(version) {
		return fmt.Errorf("versão inválida: %q", version)
	}
	return nil
}
