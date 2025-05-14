package config

import (
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

	cfg.Init(
		cfg.WithConfigPath(dir, "."),
		cfg.WithConfigName("config"),
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
