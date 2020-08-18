package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

var configPath string

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Println(err)
	}
	dir = filepath.Join(dir, AppName)
	log.Println("Making new config dir")
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		log.Println(err)
	}
	configPath = filepath.Join(dir, "config.json")
}

func LoadConfig() (*Config, error) {
	var cfg Config
	f, err := os.OpenFile(configPath, os.O_RDONLY, 0700)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, err
	}
	defer f.Close()
	log.Println("Loading config from", configPath)
	err = json.NewDecoder(f).Decode(&cfg)
	return &cfg, err
}

type Acc struct{ User, Token string }

type Config struct {
	Accs        []Acc
	SelectedAcc int
}

func (c *Config) selectedAcc() int {
	if c.SelectedAcc < 0 || c.SelectedAcc >= len(c.Accs) {
		return -1
	}
	return c.SelectedAcc
}

func (c *Config) Save() error {
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer f.Close()
	log.Println("Saving config to", configPath)
	return json.NewEncoder(f).Encode(c)
}
