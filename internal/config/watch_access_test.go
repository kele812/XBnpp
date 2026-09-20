package config

import (
	"strings"
	"testing"
)

func TestWatchAccessDockerEnvironment(t *testing.T) {
	t.Setenv("API_HOST", "https://panel.example.com")
	t.Setenv("API_KEY", "test-token")
	t.Setenv("NODE_ID", "1")
	t.Setenv("WATCH_ACCESS_URL", "https://watch.example.com")
	t.Setenv("WATCH_ACCESS_NODE", strings.Repeat("a", 48))
	t.Setenv("WATCH_ACCESS_SECRET", strings.Repeat("b", 48))
	cfg, err := LoadRoot(t.TempDir() + "/missing.yml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WatchAccess.URL != "https://watch.example.com" || cfg.WatchAccess.Node != strings.Repeat("a", 48) {
		t.Fatal("Docker environment not loaded")
	}
}
func TestWatchAccessDoesNotCrossPanelInstances(t *testing.T) {
	y := `instances:
 - panel:
     url: https://one.example.com
     token: test-one
     node_id: 1
   watch_access:
     url: https://watch.example.com
     node: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
     secret: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
 - panel:
     url: https://two.example.com
     token: test-two
     node_id: 2
`
	p := writeTemp(t, y)
	cfg, err := LoadRoot(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Instances[0].WatchAccess.Node == "" || cfg.Instances[1].WatchAccess.Node != "" {
		t.Fatal("collection credentials leaked across instances")
	}
	t.Setenv("WATCH_ACCESS_URL", "https://shared.example.com")
	if _, err = LoadRoot(p); err == nil {
		t.Fatal("global collection environment accepted for multiple panels")
	}
}
func TestWatchAccessRejectsIgnoredTopLevelInInstances(t *testing.T) {
	p := writeTemp(t, `watch_access:
 url: https://watch.example.com
instances:
 - panel:
     url: https://one.example.com
     token: test-one
     node_id: 1
`)
	if _, err := LoadRoot(p); err == nil {
		t.Fatal("silently ignored top-level collection")
	}
}
