package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/BYT0723/go-tools/cfg"
	"github.com/BYT0723/go-tools/logx"
)

var Config Configuration

type Configuration struct {
	Cookie  string  `cfg:"cookie"`
	RoomID  int64   `cfg:"room_id"`
	History History `cfg:"history"`
}

const cfgTemplate = `cookie: xxx
room_id: 0
`

func init() {
	var (
		dir     = getConfigDir("bilichat")
		cfgPath = filepath.Join(dir, "config.yaml")
	)

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(cfgPath), 0o700)
		f, err2 := os.Create(cfgPath)
		if err2 != nil {
			panic(err2)
		}
		defer f.Close()
		template.Must(template.New("config").Parse(cfgTemplate)).Execute(f, nil)
		fmt.Printf("Configuration %s has been generated, please modify the configuration in time\n", cfgPath)
		os.Exit(0)
	}

	cfg.Init(
		cfg.WithConfigName("config"),
		cfg.WithConfigPath(".", dir),
		cfg.WithConfigType("yaml"),
		cfg.WithDefaultUnMarshal(&Config),
	)

	if Config.History.Danmaku == 0 {
		Config.History.Danmaku = 1024
	}
	if Config.History.SC == 0 {
		Config.History.SC = 512
	}
	if Config.History.Gift == 0 {
		Config.History.Gift = 512
	}

	if err := logx.Init(logx.WithConf(&logx.Config{
		Name:       "bilichat",
		Ext:        "log",
		Level:      "info",
		Single:     false,
		MaxBackups: 5,
		MaxSize:    1,
		MaxAge:     7,
		Console:    false,
	})); err != nil {
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
