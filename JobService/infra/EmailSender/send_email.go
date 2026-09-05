package emailsender

import (
	"context"

	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/services/worker"
)

type EmailSender struct {
	Config *configs.Config
}

func NewEmailSender(cfg *configs.Config) worker.ISendEmail {
	return &EmailSender{
		Config: cfg,
	}
}

func (e *EmailSender) SendEmail(ctx context.Context, payload interface{}) error {
	return nil
}
