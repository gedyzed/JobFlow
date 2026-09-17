package services

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	mocks "github.com/gedyzed/JobFlow/JobService/mocks/repositories"
	"github.com/gedyzed/JobFlow/JobService/models"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_HandleWorkerResult_CompletedSuccess(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewJobConsumerService(repo, logger)

	result := workerModels.JobResult{
		JobID:      "job-1",
		UserID:     "user-1",
		JobType:    "SEND_EMAIL",
		ResultData: json.RawMessage(`{"status": "email_sent"}`),
		Status:     models.StatusCompleted,
	}

	body, err := json.Marshal(result)
	require.NoError(t, err)

	repo.On("GetJobByID", "job-1").Return(&models.Job{
		JobID:  "job-1",
		Status: models.StatusRunning,
	}, nil).Once()

	repo.On("UpdateJob", mock.MatchedBy(func(job *models.Job) bool {
		return job.JobID == "job-1" &&
			job.Status == models.StatusCompleted &&
			job.CompletedAt != nil &&
			job.Error == nil
	})).Return(nil).Once()

	err = svc.HandleWorkerResult(context.Background(), body)
	assert.NoError(t, err)
}

func Test_HandleWorkerResult_RunningSuccess(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewJobConsumerService(repo, logger)

	result := workerModels.JobResult{
		JobID:   "job-running-1",
		UserID:  "user-1",
		JobType: "SEND_EMAIL",
		Status:  models.StatusRunning,
	}

	body, err := json.Marshal(result)
	require.NoError(t, err)

	repo.On("GetJobByID", "job-running-1").Return(&models.Job{
		JobID:  "job-running-1",
		Status: models.StatusScheduled,
	}, nil).Once()

	repo.On("UpdateJob", mock.MatchedBy(func(job *models.Job) bool {
		return job.JobID == "job-running-1" &&
			job.Status == models.StatusRunning &&
			job.StartedAt != nil
	})).Return(nil).Once()

	err = svc.HandleWorkerResult(context.Background(), body)
	assert.NoError(t, err)
}

func Test_HandleWorkerResult_RunningIgnoredIfAlreadyCompleted(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewJobConsumerService(repo, logger)

	result := workerModels.JobResult{
		JobID:   "job-already-done",
		UserID:  "user-1",
		JobType: "SEND_EMAIL",
		Status:  models.StatusRunning,
	}

	body, err := json.Marshal(result)
	require.NoError(t, err)

	// Existing job is already completed
	repo.On("GetJobByID", "job-already-done").Return(&models.Job{
		JobID:  "job-already-done",
		Status: models.StatusCompleted,
	}, nil).Once()

	// UpdateJob should NOT be called
	err = svc.HandleWorkerResult(context.Background(), body)
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "UpdateJob", mock.Anything)
}

func Test_HandleWorkerResult_FailedWithError(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewJobConsumerService(repo, logger)

	errMsg := "email provider error"
	result := workerModels.JobResult{
		JobID:      "job-2",
		UserID:     "user-1",
		JobType:    "SEND_EMAIL",
		ResultData: json.RawMessage(`{"error": "email provider error"}`),
		Status:     models.StatusFailed,
		Error:      errMsg,
	}

	body, err := json.Marshal(result)
	require.NoError(t, err)

	repo.On("GetJobByID", "job-2").Return(&models.Job{
		JobID:  "job-2",
		Status: models.StatusRunning,
	}, nil).Once()

	repo.On("UpdateJob", mock.MatchedBy(func(job *models.Job) bool {
		return job.JobID == "job-2" &&
			job.Status == models.StatusFailed &&
			job.CompletedAt != nil &&
			job.Error != nil &&
			*job.Error == errMsg
	})).Return(nil).Once()

	err = svc.HandleWorkerResult(context.Background(), body)
	assert.NoError(t, err)
}

func Test_HandleWorkerResult_InvalidJSON(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewJobConsumerService(repo, logger)

	err := svc.HandleWorkerResult(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
}
