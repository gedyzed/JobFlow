package testData

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gedyzed/JobFlow/JobService/models"
)

//go:embed test_data.json
var defaultTestDataJSON []byte

type TestData struct {
	Jobs            []*models.Job            `json:"jobs"`
	IdempotencyKeys []*models.IdempotencyKey `json:"idempotency_keys"`
	OutboxEvents    []*models.Outbox         `json:"outbox_events"`
}

func LoadTestData() (*TestData, error) {
	return LoadTestDataFromBytes(defaultTestDataJSON)
}

func MustLoadTestData() *TestData {
	td, err := LoadTestData()
	if err != nil {
		panic(fmt.Sprintf("failed to load default test data: %v", err))
	}
	return td
}

func LoadTestDataFromFile(filePath string) (*TestData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading test data file %q: %w", filePath, err)
	}
	return LoadTestDataFromBytes(data)
}

func LoadTestDataFromBytes(data []byte) (*TestData, error) {
	var td TestData
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, fmt.Errorf("unmarshaling test data JSON: %w", err)
	}
	return &td, nil
}

func (td *TestData) GetJobByID(id string) (*models.Job, error) {
	for _, job := range td.Jobs {
		if job != nil && job.JobID == id {
			return job, nil
		}
	}
	return nil, fmt.Errorf("mock job with id %q not found", id)
}

func (td *TestData) GetJobByType(jobType string) (*models.Job, error) {
	for _, job := range td.Jobs {
		if job != nil && job.Type == jobType {
			return job, nil
		}
	}
	return nil, fmt.Errorf("mock job with type %q not found", jobType)
}

func (td *TestData) GetJobByStatus(status string) (*models.Job, error) {
	for _, job := range td.Jobs {
		if job != nil && job.Status == status {
			return job, nil
		}
	}
	return nil, fmt.Errorf("mock job with status %q not found", status)
}

func (td *TestData) GetJobsByStatus(status string) []*models.Job {
	var matches []*models.Job
	for _, job := range td.Jobs {
		if job != nil && job.Status == status {
			matches = append(matches, job)
		}
	}
	return matches
}

func (td *TestData) GetJobsByType(jobType string) []*models.Job {
	var matches []*models.Job
	for _, job := range td.Jobs {
		if job != nil && job.Type == jobType {
			matches = append(matches, job)
		}
	}
	return matches
}

func (td *TestData) GetIdempotencyKey(key string) (*models.IdempotencyKey, error) {
	for _, record := range td.IdempotencyKeys {
		if record != nil && record.IdempotencyKey == key {
			return record, nil
		}
	}
	return nil, fmt.Errorf("mock idempotency key %q not found", key)
}

func (td *TestData) GetOutboxByJobID(jobID string) (*models.Outbox, error) {
	for _, outbox := range td.OutboxEvents {
		if outbox != nil && outbox.JobID == jobID {
			return outbox, nil
		}
	}
	return nil, fmt.Errorf("mock outbox event for job_id %q not found", jobID)
}

func CloneJob(job *models.Job) (*models.Job, error) {
	if job == nil {
		return nil, nil
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshaling job for clone: %w", err)
	}
	var clone models.Job
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, fmt.Errorf("unmarshaling job clone: %w", err)
	}
	return &clone, nil
}
