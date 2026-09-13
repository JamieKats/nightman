package response

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

func TestLoadFindsExpectedOllamaPools(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, key := range []string{"ollama/generate", "ollama/chat", "ollama/tags", "ollama/show"} {
		pool, err := reg.Pool(key)
		if err != nil {
			t.Errorf("Pool(%q) error = %v", key, err)
			continue
		}
		if len(pool.templates) == 0 {
			t.Errorf("Pool(%q) has no templates", key)
		}
	}
}

func TestLoadEveryTemplateIsValidJSON(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for key, pool := range reg.pools {
		for i, tmpl := range pool.templates {
			var v any
			if err := json.Unmarshal(tmpl, &v); err != nil {
				t.Errorf("pool %q template %d is not valid JSON: %v", key, i, err)
			}
		}
	}
}

func TestPoolUnknownKeyIsAnError(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if _, err := reg.Pool("does-not-exist/anything"); err == nil {
		t.Fatal("Pool() error = nil, want error for an unknown key")
	}
}

func TestPoolKey(t *testing.T) {
	tests := []struct {
		path    string
		want    string
		wantErr bool
	}{
		{"templates/ollama/generate/1.json", "ollama/generate", false},
		{"templates/ollama/tags/2.json", "ollama/tags", false},
		{"templates/ollama/generate/nested/1.json", "", true}, // too deep
		{"templates/ollama/1.json", "", true},                 // too shallow
	}
	for _, tt := range tests {
		got, err := poolKey("templates", tt.path)
		if tt.wantErr {
			if err == nil {
				t.Errorf("poolKey(%q) error = nil, want error", tt.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("poolKey(%q) unexpected error: %v", tt.path, err)
		}
		if got != tt.want {
			t.Errorf("poolKey(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestLoadFromRejectsInvalidJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/ollama/generate/1.json": &fstest.MapFile{Data: []byte(`{not valid json`)},
	}
	if _, err := loadFrom(fsys, "templates"); err == nil {
		t.Fatal("loadFrom() error = nil, want error for invalid JSON")
	}
}

func TestLoadFromRejectsEmptyResult(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/readme.txt": &fstest.MapFile{Data: []byte("not a template")},
	}
	if _, err := loadFrom(fsys, "templates"); err == nil {
		t.Fatal("loadFrom() error = nil, want error when no templates are found")
	}
}

func TestLoadFromRejectsBadNesting(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/ollama/1.json": &fstest.MapFile{Data: []byte(`{}`)}, // missing endpoint level
	}
	if _, err := loadFrom(fsys, "templates"); err == nil {
		t.Fatal("loadFrom() error = nil, want error for a template not nested as <service>/<endpoint>")
	}
}

func TestLoadFromGroupsMultipleFilesIntoOnePool(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/ollama/generate/1.json": &fstest.MapFile{Data: []byte(`{"n":1}`)},
		"templates/ollama/generate/2.json": &fstest.MapFile{Data: []byte(`{"n":2}`)},
		"templates/ollama/tags/1.json":     &fstest.MapFile{Data: []byte(`{"n":3}`)},
	}
	reg, err := loadFrom(fsys, "templates")
	if err != nil {
		t.Fatalf("loadFrom() error = %v", err)
	}

	generate, err := reg.Pool("ollama/generate")
	if err != nil {
		t.Fatalf("Pool(ollama/generate) error = %v", err)
	}
	if len(generate.templates) != 2 {
		t.Errorf("ollama/generate has %d templates, want 2", len(generate.templates))
	}

	tags, err := reg.Pool("ollama/tags")
	if err != nil {
		t.Fatalf("Pool(ollama/tags) error = %v", err)
	}
	if len(tags.templates) != 1 {
		t.Errorf("ollama/tags has %d templates, want 1", len(tags.templates))
	}
}

func TestPoolRandomOnlyReturnsKnownTemplatesAndVaries(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pool, err := reg.Pool("ollama/tags")
	if err != nil {
		t.Fatalf("Pool() error = %v", err)
	}

	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		got := pool.Random()

		matched := false
		for _, tmpl := range pool.templates {
			if string(got) == string(tmpl) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("Random() returned a byte slice that isn't one of the pool's templates: %s", got)
		}
		seen[string(got)] = true
	}

	if len(pool.templates) > 1 && len(seen) < 2 {
		t.Errorf("Random() returned only %d distinct template(s) over 200 calls, want more than 1 (pool has %d)",
			len(seen), len(pool.templates))
	}
}

func TestPoolRandomSingleTemplatePool(t *testing.T) {
	pool := &Pool{key: "x/y", templates: [][]byte{[]byte(`{"a":1}`)}}
	for i := 0; i < 10; i++ {
		if string(pool.Random()) != `{"a":1}` {
			t.Fatalf("Random() on a single-template pool returned something else")
		}
	}
}
