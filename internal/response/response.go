// Package response holds the canned template responses each mocked
// service serves. v1 is a small fixed pool per endpoint (1-2 templates
// for now; expands to ~10 per docs/plans/response-breadth.md) selected
// uniformly at random per request — enough variety to avoid trivial
// byte-for-byte fingerprinting. No dynamic generation, no per-request
// customization based on the probe's content: every template is served
// byte-for-byte.
package response

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path"
	"strings"
)

//go:embed templates
var templatesFS embed.FS

// Registry holds every template pool loaded from templates, keyed by
// "<service>/<endpoint>" (e.g. "ollama/generate").
type Registry struct {
	pools map[string]*Pool
}

// Load reads and validates every template under templates/, grouping
// them into pools by their "<service>/<endpoint>" directory. It fails if
// any file isn't valid JSON, any pool would end up empty, or no
// templates are found at all — template content is fixed at compile
// time, so a problem here is a build-time mistake, not runtime data, and
// should stop the binary from starting rather than surface later against
// a real probe.
func Load() (*Registry, error) {
	return loadFrom(templatesFS, "templates")
}

// loadFrom is Load's implementation, parameterized over the filesystem
// and root directory so it can be exercised against a synthetic fs.FS in
// tests without touching the embedded templates.
func loadFrom(fsys fs.FS, root string) (*Registry, error) {
	groups := make(map[string][][]byte)

	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".json" {
			return nil
		}

		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("%s does not contain valid JSON", p)
		}

		key, err := poolKey(root, p)
		if err != nil {
			return err
		}
		groups[key] = append(groups[key], data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("response: loading templates: %w", err)
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("response: no templates found under %s/", root)
	}

	pools := make(map[string]*Pool, len(groups))
	for key, templates := range groups {
		pools[key] = &Pool{key: key, templates: templates}
	}
	return &Registry{pools: pools}, nil
}

// poolKey derives "<service>/<endpoint>" from a path relative to root,
// such as "templates/ollama/generate/1.json" under root "templates".
// embed.FS paths are always slash-separated regardless of host OS, so
// this uses package path, not path/filepath.
func poolKey(root, templatePath string) (string, error) {
	rel := strings.TrimPrefix(templatePath, root+"/")
	dir := path.Dir(rel)
	if strings.Count(dir, "/") != 1 {
		return "", fmt.Errorf("%s is not nested as %s/<service>/<endpoint>/*.json", templatePath, root)
	}
	return dir, nil
}

// Pool returns the named pool ("<service>/<endpoint>"), or an error if
// nothing was loaded for that key. Callers are expected to look up every
// pool they need during their own startup wiring, so a missing pool
// becomes a startup error rather than a surprise at request time.
func (r *Registry) Pool(key string) (*Pool, error) {
	p, ok := r.pools[key]
	if !ok {
		return nil, fmt.Errorf("response: no template pool for %q", key)
	}
	return p, nil
}

// Pool is one fake endpoint's canned response bodies, loaded and
// validated once at Load time.
type Pool struct {
	key       string
	templates [][]byte
}

// Random returns one of the pool's templates, chosen uniformly at
// random. math/rand/v2's global source is automatically seeded from the
// OS per process, so no manual seeding is required.
//
// The returned slice is the pool's own copy and must be treated as
// read-only — every caller asking for the same index shares it.
func (p *Pool) Random() []byte {
	return p.templates[rand.IntN(len(p.templates))]
}
