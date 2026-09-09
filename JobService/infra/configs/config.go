package configs

import (
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
)

type Config struct {
	DB            DBConfig
	APP           APPConfig
	RabbitMQ      RabbitMQConfig
	ObjectStorage ObjectStorageConfig
	Email         EmailConfig
}

type EmailConfig struct {
	APIKey string
	Domain string
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

type ObjectStorageConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Region          string
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
		return nil, errors.Wrap(err, "load database password")
	}

	if err := validateRequiredConfig(map[string]string{
		"DB_HOST":     dbHost,
		"DB_PORT":     dbPort,
		"DB_USER":     dbUser,
		"DB_NAME":     dbName,
		"DB_PASSWORD": dbPassword,
	}); err != nil {
		return nil, err
	}

	// RabbitMQ configuration
	rmqHost := os.Getenv("RABBITMQ_HOST")
	rmqPort := os.Getenv("RABBITMQ_PORT")
	rmqUser := os.Getenv("RABBITMQ_USER")
	rmqPassword := os.Getenv("RABBITMQ_PASSWORD")
	rmqName := os.Getenv("RABBITMQ_QUEUE_NAME")
	if rmqName == "" {
		rmqName = "job_queue"
	}
	// if err := validateRequiredConfig(map[string]string{
	// 	"RABBITMQ_HOST":     rmqHost,
	// 	"RABBITMQ_PORT":     rmqPort,
	// 	"RABBITMQ_USER":     rmqUser,
	// 	"RABBITMQ_PASSWORD": rmqPassword,
	// }); err != nil {
	// 	return nil, err
	// }

	// Object Storage configuration
	osEndpoint := os.Getenv("OBJECT_STORAGE_ENDPOINT")
	osAccessKeyID := os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID")
	osSecretAccessKey := os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY")
	osBucketName := os.Getenv("OBJECT_STORAGE_BUCKET_NAME")
	osRegion := os.Getenv("OBJECT_STORAGE_REGION")

	// if err := validateRequiredConfig(map[string]string{
	// 	"OBJECT_STORAGE_ENDPOINT":          osEndpoint,
	// 	"OBJECT_STORAGE_ACCESS_KEY_ID":     osAccessKeyID,
	// 	"OBJECT_STORAGE_SECRET_ACCESS_KEY": osSecretAccessKey,
	// 	"OBJECT_STORAGE_BUCKET_NAME":       osBucketName,
	// 	"OBJECT_STORAGE_REGION":            osRegion,
	// }); err != nil {
	// 	return nil, err
	// }

	// Email configuration
	emailAPIKey, err := readSecret("resend_api_key", "RESEND_API_KEY_FILE")
	// if err != nil {
	// 	return nil, errors.Wrap(err, "load resend api key")
	// }

	emailDomain := os.Getenv("EMAIL_DOMAIN")
	if emailDomain == "" {
		emailDomain = os.Getenv("RESEND_DOMAIN")
	}
	if emailDomain == "" {
		emailDomain = "resend.dev"
	}

	// if err := validateRequiredConfig(map[string]string{
	// 	"RESEND_API_KEY": emailAPIKey,
	// }); err != nil {
	// 	return nil, err
	// }

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
			Name:     rmqName,
		},
		ObjectStorage: ObjectStorageConfig{
			Endpoint:        osEndpoint,
			AccessKeyID:     osAccessKeyID,
			SecretAccessKey: osSecretAccessKey,
			BucketName:      osBucketName,
			Region:          osRegion,	
		},
		Email: EmailConfig{
			APIKey: emailAPIKey,
			Domain: emailDomain,
		},
	}, nil
}

func validateRequiredConfig(config map[string]string) error {
	for name, value := range config {
		if value == "" {
			return errors.Errorf("required configuration %s is not set", name)
		}
	}

	return nil
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
