// Package presets holds per-model TV settings (backlight macro). Presets are
// built into the exe and can be refreshed from the GitHub repository.
package presets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Repo is the GitHub repository whose presets/ folder is the source.
const (
	Repo = "angellfear/tvtray"
	Dir  = "presets"
)

type Backlight struct {
	OpenKeys    []string `json:"openKeys"`
	CloseKeys   []string `json:"closeKeys"`
	Max         int      `json:"max"`
	NavDelayMs  int      `json:"navDelayMs"`
	StepDelayMs int      `json:"stepDelayMs"`
}

type Preset struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Models       []string  `json:"models"`
	ModelPattern string    `json:"modelPattern"`
	Backlight    Backlight `json:"backlight"`
}

func (p *Preset) validate() error {
	if p.ID == "" || p.Name == "" {
		return fmt.Errorf("preset without id/name")
	}
	if p.ModelPattern != "" {
		if _, err := regexp.Compile(p.ModelPattern); err != nil {
			return fmt.Errorf("%s: modelPattern: %w", p.ID, err)
		}
	}
	if len(p.Backlight.OpenKeys) == 0 || p.Backlight.Max <= 0 {
		return fmt.Errorf("%s: backlight.openKeys/max required", p.ID)
	}
	return nil
}

func parse(data []byte) (*Preset, error) {
	var p Preset
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func readDir(fsys fs.FS, dir string) map[string]*Preset {
	out := map[string]*Preset{}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			continue
		}
		if p, err := parse(data); err == nil {
			out[p.ID] = p
		}
	}
	return out
}

// Load merges built-in presets with downloaded ones (downloaded win).
func Load(builtin fs.FS, cacheDir string) []Preset {
	all := readDir(builtin, Dir)
	for id, p := range readDir(os.DirFS(cacheDir), ".") {
		all[id] = p
	}
	out := make([]Preset, 0, len(all))
	for _, p := range all {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Match returns the best preset for a model: exact model first, then pattern.
func Match(list []Preset, model string) *Preset {
	model = strings.ToUpper(strings.TrimSpace(model))
	for i := range list {
		for _, m := range list[i].Models {
			if strings.EqualFold(m, model) {
				return &list[i]
			}
		}
	}
	for i := range list {
		if list[i].ModelPattern == "" {
			continue
		}
		if re, err := regexp.Compile(list[i].ModelPattern); err == nil && re.MatchString(model) {
			return &list[i]
		}
	}
	return nil
}

// Download fetches presets/*.json from GitHub into cacheDir, replacing what
// was there. Returns the number of presets saved.
func Download(ctx context.Context, cacheDir string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var listing []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := getJSON(ctx, "https://api.github.com/repos/"+Repo+"/contents/"+Dir, &listing); err != nil {
		return 0, err
	}

	tmp := cacheDir + ".new"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return 0, err
	}
	n := 0
	for _, f := range listing {
		if f.Type != "file" || !strings.HasSuffix(f.Name, ".json") || f.DownloadURL == "" {
			continue
		}
		data, err := get(ctx, f.DownloadURL)
		if err != nil {
			os.RemoveAll(tmp)
			return 0, fmt.Errorf("%s: %w", f.Name, err)
		}
		if _, err := parse(data); err != nil {
			continue // skip broken files, keep the rest
		}
		if err := os.WriteFile(filepath.Join(tmp, filepath.Base(f.Name)), data, 0o644); err != nil {
			os.RemoveAll(tmp)
			return 0, err
		}
		n++
	}
	os.RemoveAll(cacheDir)
	if err := os.Rename(tmp, cacheDir); err != nil {
		return 0, err
	}
	return n, nil
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "TvTray")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func getJSON(ctx context.Context, url string, v any) error {
	data, err := get(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
