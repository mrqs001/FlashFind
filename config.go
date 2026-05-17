package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const cfgFile = ".flashfind.json"

type config struct {
	W, H       float32
	LastFolder string
}

func loadCfg() config {
	home, _ := os.UserHomeDir()
	file := filepath.Join(home, cfgFile)

	def := config{W: 1150, H: 780}
	b, err := os.ReadFile(file)
	if err != nil {
		return def
	}
	var c config
	if json.Unmarshal(b, &c) != nil {
		return def
	}
	return c
}

func saveCfg(c config) {
	home, _ := os.UserHomeDir()
	_ = os.WriteFile(
		filepath.Join(home, cfgFile),
		must(json.MarshalIndent(c, "", "  ")),
		0o600,
	)
}

func must[T any](v T, _ error) T { return v }
