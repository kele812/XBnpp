package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kele812/XBnpp/internal/watchaccess"
	"gopkg.in/yaml.v3"
)

func watchTestArgs() []string {
	return []string{"--watch-url", "https://watch.example.com", "--watch-node", strings.Repeat("a", 48), "--watch-secret", strings.Repeat("b", 48)}
}

func TestWatchArgsRejectInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"--watch-url"}, {"--watch-url", "https://watch.example.com"},
		append(watchTestArgs(), "--watch-secret", "bad"),
		{"--watch-url", "http://watch.example.com", "--watch-node", strings.Repeat("a", 48), "--watch-secret", strings.Repeat("b", 48)},
		{"--watch-url", "https://watch.example.com", "--watch-node", "bad", "--watch-secret", "secret-must-not-leak"},
	} {
		_, _, err := parseWatchArgs(args)
		if err == nil || strings.Contains(err.Error(), "secret-must-not-leak") {
			t.Fatal("invalid arguments accepted or secret exposed")
		}
	}
}

func TestWatchArgsMultipleOrigins(t *testing.T) {
	args := append([]string{"--watch-url", "https://backup.example.com"}, watchTestArgs()...)
	_, c, err := parseWatchArgs(args)
	if err != nil || c == nil || len(c.URLs) != 2 || c.URLs[0] != "https://backup.example.com" || c.URLs[1] != "https://watch.example.com" {
		t.Fatalf("multiple origins not preserved in order: %v, %#v", err, c)
	}
}

func TestSetWatchPreservesOtherFieldsAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	input := `instances:
  - id: first
    panel: {url: https://a.example.com, token: keep-inline-token}
    machine: {machine_id: 3, token_env: KEEP_ENV}
    custom_extension: {keep: true}
  - id: second
    panel: {url: https://b.example.com}
    watch_access: {url: https://old.example.com, node: keep-node, secret: keep-secret}
`
	os.WriteFile(path, []byte(input), 0600)
	_, c, _ := parseWatchArgs(watchTestArgs())
	if err := setWatchConfig(path, "first", *c); err != nil {
		t.Fatal(err)
	}
	backup, _ := os.ReadFile(path + ".watch.bak")
	if string(backup) != input {
		t.Fatal("backup not exact")
	}
	data, _ := os.ReadFile(path)
	var before, after map[string]any
	yaml.Unmarshal([]byte(input), &before)
	yaml.Unmarshal(data, &after)
	first := after["instances"].([]any)[0].(map[string]any)
	if first["watch_access"].(map[string]any)["secret"] != c.Secret {
		t.Fatal("collector not saved")
	}
	delete(first, "watch_access")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated configuration changed")
	}
	if err := setWatchConfig(path, "missing", *c); err == nil {
		t.Fatal("unknown instance accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != string(data) {
		t.Fatal("unknown instance modified file")
	}
}

func TestBindInitWatchAndRebindPreservesIt(t *testing.T) {
	for _, mode := range []string{"node", "machine"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			base := []string{"--mode", mode, "--panel-url", "https://panel.example.com", "--" + mode + "-id", "3", "--kernel", "xray", "--config", path, "--output", path}
			if err := runConfigInit(append(append([]string{}, base...), watchTestArgs()...)); err != nil {
				t.Fatal(err)
			}
			if err := runConfigInit(base); err != nil {
				t.Fatal(err)
			}
			root, err := loadWritableRootConfig(path)
			if err != nil || len(root.Instances) != 1 || root.Instances[0].Kernel.Type != "xray" || root.Instances[0].WatchAccess.Secret != strings.Repeat("b", 48) {
				t.Fatal("rebind lost collector or kernel")
			}
			before, _ := os.ReadFile(path)
			if err := runConfigInit(append(base, "--watch-secret", "bad")); err == nil {
				t.Fatal("partial collector settings accepted")
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("invalid arguments modified file")
			}
			second := append([]string{}, base...)
			second[3] = "https://second.example.com"
			secondWatch := watchTestArgs()
			secondWatch[5] = strings.Repeat("c", 48)
			if err := runConfigInit(append(second, secondWatch...)); err != nil {
				t.Fatal(err)
			}
			root, err = loadWritableRootConfig(path)
			if err != nil || len(root.Instances) != 2 || root.Instances[0].WatchAccess.Secret != strings.Repeat("b", 48) || root.Instances[1].WatchAccess.Secret != strings.Repeat("c", 48) {
				t.Fatal("panel collector isolation lost")
			}
		})
	}
}

func TestSetWatchRejectsAmbiguousOrMalformedConfig(t *testing.T) {
	for _, input := range []string{"instances: [", "instances:\n  - id: same\n  - id: same\n", "instances:\n  - id: same\n    id: duplicate\n"} {
		path := filepath.Join(t.TempDir(), "config.yml")
		os.WriteFile(path, []byte(input), 0600)
		c := watchaccess.Config{URL: "https://watch.example.com", Node: strings.Repeat("a", 48), Secret: strings.Repeat("b", 48)}
		if err := setWatchConfig(path, "same", c); err == nil {
			t.Fatal("bad config accepted")
		}
		data, _ := os.ReadFile(path)
		if string(data) != input {
			t.Fatal("bad config overwritten")
		}
	}
}
