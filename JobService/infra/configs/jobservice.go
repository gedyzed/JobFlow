package configs

import "github.com/cockroachdb/errors"

type JobServiceConfig struct {
	APP      APPConfig
	DB       DBConfig
	RabbitMQ RabbitMQConfig
}

func LoadJobServiceConfig() (*JobServiceConfig, error) {
	appCfg := LoadAPPConfig()
	dbCfg, err := LoadDBConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load jobservice database config")
	}

	rmqCfg, err := LoadRabbitMQConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load jobservice rabbitmq config")
	}

	return &JobServiceConfig{
		APP:      appCfg,
		DB:       *dbCfg,
		RabbitMQ: *rmqCfg,
	}, nil
}
