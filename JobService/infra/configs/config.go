package configs

import (
	"fmt"
	"os"
	"strings"
	
)

type Config struct {
	DB DBConfig 
	APP APPConfig
}

type APPConfig struct {
	Port string
}

type DBConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func LoadConfig() (*Config, error) {
	// App configuration
	Port := os.Getenv("PORT")

	// DB configuration
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbName := os.Getenv("DB_NAME")
	dbPassword, err := readSecret("DB_PASSWORD", "DB_PASSWORD_FILE")
	if err != nil {
		return nil, err
	}

	for name, value := range map[string]string{
		"DB_HOST":     dbHost,
		"DB_PORT":     dbPort,
		"DB_USER":     dbUser,
		"DB_NAME":     dbName,
		"DB_PASSWORD": dbPassword,
	} {
		if value == "" {
			return nil, fmt.Errorf("required configuration %s is not set", name)
		}
	}

	return &Config{
		APP: APPConfig{
			Port: Port,
		},
		DB: DBConfig{
			URL:      fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName),
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			Name:     dbName,
		},
	}, nil
}


func readSecret(valueName, fileName string) (string, error) {
	if value := os.Getenv(valueName); value != "" {
		return value, nil
	}

	path := os.Getenv(fileName)
	if path == "" {
		return "", nil
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", fileName, err)
	}

	return strings.TrimSpace(string(secret)), nil
}



