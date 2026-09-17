package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	modelsMocks "github.com/gedyzed/JobFlow/JobService/mocks/models"
	repoMocks "github.com/gedyzed/JobFlow/JobService/mocks/repositories"
	workerMocks "github.com/gedyzed/JobFlow/JobService/mocks/worker"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestWorker(t *testing.T) (*repoMocks.MockIWorkerRepository, *workerMocks.MockISendEmail, func(context.Context, []byte) error) {
	mockRepo := repoMocks.NewMockIWorkerRepository(t)
	mockRMQ := modelsMocks.NewMockIRMQClient(t)
	mockEmail := workerMocks.NewMockISendEmail(t)
	mockS3 := modelsMocks.NewMockIObjectStorage(t)
	mockConsumer := modelsMocks.NewMockConsumer(t)

	var handler func(context.Context, []byte) error
	mockRMQ.On("NewConsumer", mock.Anything, "job_queue").Return(mockConsumer, nil)
	mockConsumer.On("Consume", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		handler = args.Get(1).(func(context.Context, []byte) error)
	}).Return(nil)
	mockConsumer.On("Close", mock.Anything).Return(nil)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewWorkerService(mockRepo, logger, mockRMQ, mockEmail, mockS3)
	_ = svc.StartWorker(context.Background())

	return mockRepo, mockEmail, handler
}

func Test_ConsumeJobs_SendEmailSuccess(t *testing.T) {
	mockRepo, mockEmail, handler := setupTestWorker(t)

	jobJSON := []byte(`{"job_id": "job-1", "user_id": "user-1", "type": "SEND_EMAIL", "payload": {"subject": "Test"}}`)

	// Deduplication check: no existing result found
	mockRepo.On("GetJobResultRecord", mock.Anything, "job-1").Return(nil, nil).Once()
	// Record running status in job_results table
	mockRepo.On("SaveJobResultStatus", mock.Anything, "job-1", "running", mock.Anything).Return(nil).Once()
	mockEmail.On("SendEmail", mock.Anything, mock.Anything).Return(nil).Once()
	// Final result saved with outbox event
	mockRepo.On("SaveJobResult", mock.Anything, "job-1", mock.MatchedBy(func(res workerModels.JobResult) bool {
		return res.Status == "completed" && res.JobID == "job-1"
	})).Return(nil).Once()

	err := handler(context.Background(), jobJSON)
	assert.NoError(t, err)
}

func Test_ConsumeJobs_SendEmailFailure(t *testing.T) {
	mockRepo, mockEmail, handler := setupTestWorker(t)

	jobJSON := []byte(`{"job_id": "job-2", "user_id": "user-1", "type": "SEND_EMAIL", "payload": {}}`)

	mockRepo.On("GetJobResultRecord", mock.Anything, "job-2").Return(nil, nil).Once()
	mockRepo.On("SaveJobResultStatus", mock.Anything, "job-2", "running", mock.Anything).Return(nil).Once()
	mockEmail.On("SendEmail", mock.Anything, mock.Anything).Return(errors.New("provider timeout")).Once()
	mockRepo.On("SaveJobResult", mock.Anything, "job-2", mock.MatchedBy(func(res workerModels.JobResult) bool {
		return res.Status == "failed" && res.JobID == "job-2" && res.Error == "provider timeout"
	})).Return(nil).Once()

	err := handler(context.Background(), jobJSON)
	assert.NoError(t, err)
}

func Test_ConsumeJobs_Deduplication(t *testing.T) {
	mockRepo, mockEmail, handler := setupTestWorker(t)

	jobJSON := []byte(`{"job_id": "job-3", "user_id": "user-1", "type": "SEND_EMAIL", "payload": {}}`)

	// Existing completed record in job_results table -> skips processing
	completedResult := &workerModels.JobResult{
		JobID:  "job-3",
		Status: "completed",
	}
	mockRepo.On("GetJobResultRecord", mock.Anything, "job-3").Return(completedResult, nil).Once()

	err := handler(context.Background(), jobJSON)
	assert.NoError(t, err)
	// SendEmail and SaveJobResult must NOT be called
	mockEmail.AssertNotCalled(t, "SendEmail", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "SaveJobResult", mock.Anything, mock.Anything, mock.Anything)
}

func Test_ConsumeJobs_InvalidMessage(t *testing.T) {
	mockRepo, mockEmail, handler := setupTestWorker(t)

	// Missing job_id or type (e.g. echo of result message) -> safely ignored
	invalidJSON := []byte(`{"status": "completed"}`)

	err := handler(context.Background(), invalidJSON)
	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "GetJobResultRecord", mock.Anything, mock.Anything)
	mockEmail.AssertNotCalled(t, "SendEmail", mock.Anything, mock.Anything)
}

