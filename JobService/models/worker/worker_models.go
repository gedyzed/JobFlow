package worker

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobResult struct {
	JobID      string // think about using jobId as forieng key to job table
	ResultId   string `json:"result_id" gorm:"primaryKey"`
	ResultKey  string
	JobType    string
	ResultData json.RawMessage
	Status     string
	Error      string

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
