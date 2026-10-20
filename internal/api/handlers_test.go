package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
)

func TestHandleListVersionsFiltersJavaProviders(t *testing.T) {
	client := &sdkman.MockClient{
		VersionsResult: map[string][]sdkman.Version{
			"java": {
				{Version: "21.0.3-amzn"},
				{Version: "21.0.3-tem"},
			},
		},
	}
	h, err := newHandler(client, nil)
	if err != nil {
		t.Fatalf("newHandler() error = %v", err)
	}

	r := httptest.NewRequest("GET", "/candidates/java/versions", nil)
	r.SetPathValue("name", "java")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.handleListVersions(w, r)

	body := w.Body.String()
	for _, want := range []string{
		`id="java-version-search"`,
		`data-java-version-row`,
		"21.0.3-amzn",
		"21.0.3-tem",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Java versions response does not contain %q", want)
		}
	}
}

func TestHandleListVersionsDoesNotShowJavaProviderFilterForOtherCandidates(t *testing.T) {
	h, err := newHandler(&sdkman.MockClient{}, nil)
	if err != nil {
		t.Fatalf("newHandler() error = %v", err)
	}

	r := httptest.NewRequest("GET", "/candidates/maven/versions", nil)
	r.SetPathValue("name", "maven")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	h.handleListVersions(w, r)

	if strings.Contains(w.Body.String(), `id="java-version-search"`) {
		t.Error("non-Java versions response contains the Java provider filter")
	}
}

func TestHandleIndexPlacesProgressBeforeVersions(t *testing.T) {
	h, err := newHandler(&sdkman.MockClient{}, nil)
	if err != nil {
		t.Fatalf("newHandler() error = %v", err)
	}

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.handleIndex(w, r)

	body := w.Body.String()
	progressIndex := strings.Index(body, `id="job-section"`)
	versionsIndex := strings.Index(body, `id="versions-section"`)
	if progressIndex == -1 || versionsIndex == -1 {
		t.Fatalf("index response is missing progress or versions section")
	}
	if progressIndex > versionsIndex {
		t.Errorf("progress section index %d is after versions section index %d", progressIndex, versionsIndex)
	}
}

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
