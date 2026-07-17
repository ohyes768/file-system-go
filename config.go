package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port         string `yaml:"port"`
		ReadTimeout  int    `yaml:"read_timeout"`
		WriteTimeout int    `yaml:"write_timeout"`
	} `yaml:"server"`
	Storage struct {
		AudioDir    string `yaml:"audio_dir"`
		MaxUploadMB int    `yaml:"max_upload_mb"`
	} `yaml:"storage"`
	UI struct {
		Password    string `yaml:"password"`
		SessionDays int    `yaml:"session_days"`
	} `yaml:"ui"`
	Logging struct {
		Level  string `yaml:"level"`
		LogDir string `yaml:"log_dir"`
	} `yaml:"logging"`
}

var config Config

func loadConfig() error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		consoleLogger.Printf("警告: 无法读取配置文件，使用默认配置: %v", err)
		setDefaultConfig()
		return nil
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("配置文件解析失败: %v", err)
	}
	if config.UI.SessionDays <= 0 {
		config.UI.SessionDays = 7
	}
	if config.Storage.MaxUploadMB <= 0 {
		config.Storage.MaxUploadMB = 1024
	}
	consoleLogger.Println("配置文件加载成功")
	return nil
}

func setDefaultConfig() {
	config.Server.Port = "8000"
	config.Server.ReadTimeout = 3600
	config.Server.WriteTimeout = 3600
	config.Storage.AudioDir = "./audio_files"
	config.Storage.MaxUploadMB = 1024
	config.UI.Password = "change-me"
	config.UI.SessionDays = 7
	config.Logging.Level = "INFO"
	config.Logging.LogDir = "./logs"
}
