package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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
	cfgPath := filepath.Join(getConfigDir("bilichat"), "config.json")

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

func getConfigDir(appName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, appName)
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", appName)
	default: // linux and others
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		return filepath.Join(configHome, appName)
	}
}
