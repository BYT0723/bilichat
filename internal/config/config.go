package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BYT0723/go-tools/cfg"
	"github.com/BYT0723/go-tools/logx"
)

var Config Configuration

type Configuration struct {
	Log    *logx.Config
	Cookie string
	RoomId int64
}

func init() {
	dir, err := getConfigDir()
	if err != nil {
		panic(err)
	}
	cfgPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Printf("cfgPath: %v\n", cfgPath)
		_ = os.MkdirAll(filepath.Dir(cfgPath), 0700)
		_ = os.WriteFile(cfgPath, []byte("{\n\t\"Cookie\": \"\",\n\t\"RoomId\": 0\n}"), 0700)

		fmt.Printf("Configuration %s has been generated, please modify the configuration in time\n", cfgPath)
		os.Exit(0)
	}

	cfg.Init(
		cfg.WithConfigFile(cfgPath),
		cfg.WithConfigType("json"),
		cfg.WithDefaultUnMarshal(&Config),
	)
	if Config.Log == nil {
		Config.Log = &logx.Config{
			Name:       "bilichat",
			Ext:        "log",
			Level:      "info",
			Single:     false,
			MaxBackups: 5,
			MaxSize:    1,
			MaxAge:     7,
			Console:    false,
		}
	}
	if err := logx.Init(logx.WithConf(Config.Log)); err != nil {
		panic(err)
	}
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bilichat"), nil
}
