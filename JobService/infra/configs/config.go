package configs

import (
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
)

type Config struct {
	DB DBConfig 
	APP APPConfig
	RabbitMQ RabbitMQConfig
}

type APPConfig struct {
	Port string
}

type RabbitMQConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
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

	// RabbitMQ configuration
	rmqHost := os.Getenv("RABBITMQ_HOST")
	rmqPort := os.Getenv("RABBITMQ_PORT")
	rmqUser := os.Getenv("RABBITMQ_USER")
	rmqPassword := os.Getenv("RABBITMQ_PASSWORD")

	if err != nil {
		return nil, errors.Wrap(err, "load database password")
	}

	for name, value := range map[string]string{
		"DB_HOST":     dbHost,
		"DB_PORT":     dbPort,
		"DB_USER":     dbUser,
		"DB_NAME":     dbName,
		"DB_PASSWORD": dbPassword,
	} {
		if value == "" {
			return nil, errors.Errorf("required configuration %s is not set", name)
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
		RabbitMQ: RabbitMQConfig{
			URL:      fmt.Sprintf("amqp://%s:%s@%s:%s/", rmqUser, rmqPassword, rmqHost, rmqPort),
			Host:     rmqHost,
			Port:     rmqPort,
			User:     rmqUser,
			Password: rmqPassword,
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
		return "", errors.Wrapf(err, "read %s", fileName)
	}

	return strings.TrimSpace(string(secret)), nil
}



