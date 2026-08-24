package infra

import (
	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DBInit(dbConfig configs.DBConfig) (*gorm.DB, error) {

	db, err := gorm.Open(postgres.Open(dbConfig.URL), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	
	return db, nil
}