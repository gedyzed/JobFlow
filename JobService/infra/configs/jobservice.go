package configs

import "github.com/cockroachdb/errors"

type JobServiceConfig struct {
	APP APPConfig
	DB  DBConfig
}

func LoadJobServiceConfig() (*JobServiceConfig, error) {
	appCfg := LoadAPPConfig()
	dbCfg, err := LoadDBConfig()
	if err != nil {
		return nil, errors.Wrap(err, "load jobservice database config")
	}

	return &JobServiceConfig{
		APP: appCfg,
		DB:  *dbCfg,
	}, nil
}
