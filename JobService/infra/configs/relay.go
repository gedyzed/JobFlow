package configs

import "github.com/cockroachdb/errors"

type RelayConfig struct {
	DB       DBConfig
	RabbitMQ RabbitMQConfig
}

func LoadRelayConfig() (*RelayConfig, error) {
	dbCfg, err := LoadDBConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load relay database config")
	}

	rmqCfg, err := LoadRabbitMQConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load relay rabbitmq config")
	}

	return &RelayConfig{
		DB:       *dbCfg,
		RabbitMQ: *rmqCfg,
	}, nil
}
