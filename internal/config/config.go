package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DBUrl        string `json:"db_url"`
	CurrUsername string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	// Get Home Directory
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Build file path
	fullPath := filepath.Join(homedir, configFileName)

	return fullPath, nil
}

func write(cfg *Config) error {
	// Get file path
	fullPath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	// Convert config to bytes
	fileBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	// Write bytes to file
	if err := os.WriteFile(fullPath, fileBytes, 0600); err != nil {
		return err
	}

	return nil
}

func Read() (Config, error) {
	// Get full file path
	fullPath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}

	// Read file
	fileBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return Config{}, err
	}

	//Unmarshal into Config
	con := Config{}
	err = json.Unmarshal(fileBytes, &con)
	if err != nil {
		return Config{}, err
	}

	return con, nil
}

func (c *Config) SetUser(username string) error {
	c.CurrUsername = username
	err := write(c)
	if err != nil {
		return err
	}

	return nil
}
