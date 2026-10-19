package sdkman

import "regexp"

// reANSI remove sequências de escape ANSI/VT100 das strings de log.
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI remove códigos ANSI de uma string.
func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}
