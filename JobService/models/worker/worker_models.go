package worker

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobResult struct {
	UserID     string  `json:"user_id"`
	JobID      string  `json:"job_id" gorm:"index"`
	ResultId   string `json:"result_id" gorm:"primaryKey"`
	ResultKey  string `json:"result_key"`
	JobType    string `json:"job_type"`
	ResultData json.RawMessage `json:"result_data" gorm:"type:jsonb"`
	Status     string `json:"status"`
	Error      string `json:"error"`

	gorm.Model
}

func (result *JobResult) BeforeCreate(tx *gorm.DB) error {
	if result.ResultId == "" {
		resultID, err := uuid.NewV7()
		if err != nil {
			return errors.Wrap(err, "generate result UUIDv7")
		}
		result.ResultId = resultID.String()
	}
	return nil
}

type EmailTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SendEmailPayload struct {
	AppName string     `json:"app_name,omitempty"`
	From    string     `json:"from,omitempty"`
	To      []string   `json:"to"`
	Subject string     `json:"subject"`
	Body    string     `json:"body,omitempty"`
	Html    string     `json:"html,omitempty"`
	Text    string     `json:"text,omitempty"`
	Cc      []string   `json:"cc,omitempty"`
	Bcc     []string   `json:"bcc,omitempty"`
	ReplyTo string     `json:"reply_to,omitempty"`
	Tags    []EmailTag `json:"tags,omitempty"`
}
