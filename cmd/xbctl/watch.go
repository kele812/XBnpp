package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kele812/XBnpp/internal/watchaccess"
	"gopkg.in/yaml.v3"
)

// Extract and validate before any configuration or credential writes.
func parseWatchArgs(args []string) ([]string, *watchaccess.Config, error) {
	c := watchaccess.Config{}
	seen := map[string]bool{}
	var watchURLs []string
	var rest []string
	for i := 0; i < len(args); i++ {
		key := args[i]
		if !strings.HasPrefix(key, "--watch-") {
			rest = append(rest, key)
			continue
		}
		if key != "--watch-url" && key != "--watch-node" && key != "--watch-secret" {
			return nil, nil, errors.New("unknown watch option; use --watch-url, --watch-node and --watch-secret")
		}
		if (seen[key] && key != "--watch-url") || i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") || args[i+1] == "" {
			return nil, nil, fmt.Errorf("%s requires one non-empty value and must not be repeated", key)
		}
		seen[key] = true
		i++
		switch key {
		case "--watch-url":
			watchURLs = append(watchURLs, args[i])
		case "--watch-node":
			c.Node = args[i]
		case "--watch-secret":
			c.Secret = args[i]
		}
	}
	if len(seen) == 0 {
		return rest, nil, nil
	}
	if len(seen) != 3 {
		return nil, nil, errors.New("provide --watch-url, --watch-node and --watch-secret together")
	}
	if len(watchURLs) == 1 { c.URL = watchURLs[0] } else { c.URLs = watchURLs }
	if _, err := watchaccess.New(c); err != nil {
		return nil, nil, err
	}
	return rest, &c, nil
}

func runBindSetWatch(args []string) error {
	rest, c, err := parseWatchArgs(args)
	if err != nil {
		return err
	}
	if c == nil || len(rest) != 2 || rest[0] != "--instance-id" || rest[1] == "" {
		return errors.New("usage: xbctl bind set-watch --instance-id ID --watch-url URL --watch-node ID --watch-secret SECRET")
	}
	if err = setWatchConfig(defaultConfigPath, rest[1], *c); err != nil {
		return err
	}
	fmt.Println("Collector configuration saved; backup: " + defaultConfigPath + ".watch.bak")
	if err = runCommand("systemctl", "restart", serviceName); err != nil {
		return fmt.Errorf("configuration saved but service restart failed; inspect journalctl -u %s: %w", serviceName, err)
	}
	fmt.Println("Collector configured successfully")
	return nil
}

func yamlValue(n *yaml.Node, key string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// Edit the YAML tree rather than reserialize a partial struct: unrelated
// fields, inline credentials and extension settings must survive unchanged.
func setWatchConfig(path, instanceID string, c watchaccess.Config) error {
	if _, err := watchaccess.New(c); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(data, &doc); err != nil {
		return errors.New("invalid config YAML; no changes saved")
	}
	var check any
	if err = doc.Decode(&check); err != nil || len(doc.Content) != 1 {
		return errors.New("invalid config YAML; no changes saved")
	}
	instances := yamlValue(doc.Content[0], "instances")
	if instances == nil || instances.Kind != yaml.SequenceNode {
		return errors.New("config must contain an instances list; no changes saved")
	}
	var target *yaml.Node
	for _, inst := range instances.Content {
		id := yamlValue(inst, "id")
		if id != nil && id.Value == instanceID {
			if target != nil {
				return errors.New("duplicate instance ID; no changes saved")
			}
			target = inst
		}
	}
	if target == nil {
		return errors.New("instance ID not found; no changes saved")
	}
	var value yaml.Node
	if err = value.Encode(c); err != nil {
		return err
	}
	if old := yamlValue(target, "watch_access"); old != nil {
		*old = value
	} else {
		target.Content = append(target.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "watch_access"}, &value)
	}
	updated, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err = atomicPrivateWrite(path+".watch.bak", data); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	return atomicPrivateWrite(path, updated)
}

func atomicPrivateWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".xbctl-watch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
