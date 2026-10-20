package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestVersion(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
		wantErr     bool
	}{
		{
			name:        "JSON",
			contentType: "application/json",
			body:        `{"version":"11.0.14.1-jbr"}`,
			want:        "11.0.14.1-jbr",
		},
		{
			name:        "HTMX form",
			contentType: "application/x-www-form-urlencoded",
			body:        "version=11.0.14.1-jbr",
			want:        "11.0.14.1-jbr",
		},
		{
			name:        "malformed JSON",
			contentType: "application/json",
			body:        `{"version":`,
			wantErr:     true,
		},
		{
			name:        "unsupported content type",
			contentType: "text/plain",
			body:        "version=11.0.14.1-jbr",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)

			got, err := requestVersion(r)
			if (err != nil) != tt.wantErr {
				t.Fatalf("requestVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("requestVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
