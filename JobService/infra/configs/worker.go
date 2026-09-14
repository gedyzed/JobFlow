package configs

import "github.com/cockroachdb/errors"

type WorkerConfig struct {
	DB            DBConfig
	RabbitMQ      RabbitMQConfig
	ObjectStorage ObjectStorageConfig
	Email         EmailConfig
}

func LoadWorkerConfig() (*WorkerConfig, error) {
	dbCfg, err := LoadDBConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load worker database config")
	}

	rmqCfg, err := LoadRabbitMQConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load worker rabbitmq config")
	}

	osCfg, err := LoadObjectStorageConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load worker object storage config")
	}

	emailCfg, err := LoadEmailConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load worker email config")
	}

	return &WorkerConfig{
		DB:            *dbCfg,
		RabbitMQ:      *rmqCfg,
		ObjectStorage: *osCfg,
		Email:         *emailCfg,
	}, nil
}
