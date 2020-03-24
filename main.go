package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

const serverCfgFile = "server.json"

type serverConfig struct {
	ListenOn  string `json:"listenOn"`
	ConfigDir string `json:"configDir"`
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func main() {

	cfg := &serverConfig{}
	if fileExists(serverCfgFile) {
		f, err := os.Open(serverCfgFile)
		if err != nil {
			log.Fatalf("failed to read %s: %v", serverCfgFile, err)
		}

		defer f.Close()

		bytes, _ := ioutil.ReadAll(f)
		json.Unmarshal(bytes, cfg)
	} else {
		log.Printf("%s not found, using default config")
		cfg.ConfigDir = "./cfg"
		cfg.ListenOn = ":8081"
		log.Printf("%+v", cfg)
	}

	Start(cfg.ConfigDir, cfg.ListenOn)
}
