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

	jobJSON := []byte(`{"job_id": "job-1", "type": "SEND_EMAIL", "payload": {"subject": "Test"}}`)

	mockRepo.On("UpdateJobStatus", mock.Anything, "job-1", "running").Return(nil).Once()
	mockEmail.On("SendEmail", mock.Anything, mock.Anything).Return(nil).Once()
	mockRepo.On("UpdateJobStatus", mock.Anything, "job-1", "completed").Return(nil).Once()
	mockRepo.On("SaveJobResult", mock.Anything, "job-1", `{"status": "email_sent"}`).Return(nil).Once()

	err := handler(context.Background(), jobJSON)
	assert.NoError(t, err)
}

func Test_ConsumeJobs_SendEmailFailure(t *testing.T) {
	mockRepo, mockEmail, handler := setupTestWorker(t)

	jobJSON := []byte(`{"job_id": "job-2", "type": "SEND_EMAIL", "payload": {}}`)

	mockRepo.On("UpdateJobStatus", mock.Anything, "job-2", "running").Return(nil).Once()
	mockEmail.On("SendEmail", mock.Anything, mock.Anything).Return(errors.New("provider timeout")).Once()
	mockRepo.On("UpdateJobStatus", mock.Anything, "job-2", "failed").Return(nil).Once()
	mockRepo.On("SaveJobResult", mock.Anything, "job-2", mock.Anything).Return(nil).Once()

	err := handler(context.Background(), jobJSON)
	assert.NoError(t, err)
}


