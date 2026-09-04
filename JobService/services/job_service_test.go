package services

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/cockroachdb/errors"
	mocks "github.com/gedyzed/JobFlow/JobService/mocks/repositories"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/testData"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func Test_CreateJob_WithValidData(t *testing.T) {
	// Arrange
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	logger := newDiscardLogger()
	service := NewJobService(repo, logger)

	idempotencyKey := "unique-idempotency-key"

	expectedJob, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b918")
	require.NoError(t, err)

	job := &models.Job{
		UserID:  expectedJob.UserID,
		Type:    expectedJob.Type,
		Payload: expectedJob.Payload,
	}

	responsePayload, err := json.Marshal(expectedJob)
	require.NoError(t, err)

	repo.On("CreateIdempotencyKey", mock.Anything).Return(nil)
	repo.On("CreateJob", job).Return(expectedJob, nil)
	repo.On("UpdateIdempotencyKeyResponse", idempotencyKey, json.RawMessage(responsePayload)).Return(nil)

	// Act
	res, err := service.CreateJob(job, idempotencyKey)

	// Assert
	require.NoError(t, err, "CreateJob should not return an error")
	assert.NotNil(t, res, "Created job should not be nil")
	assert.Equal(t, expectedJob.JobID, res.JobID)
	assert.Equal(t, models.StatusPending, res.Status)
	assert.Equal(t, models.JobTypeSendEmail, res.Type)
	assert.Equal(t, expectedJob.UserID, res.UserID)
	assert.Equal(t, expectedJob.Payload, res.Payload)
}

func Test_CreateJob_MissingIdempotencyKey(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	job, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b918")
	require.NoError(t, err)

	res, err := service.CreateJob(job, "")
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, models.ErrMissingIdempotencyKey))
}

func Test_CreateJob_IdempotentReplay(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	idempotencyRecord, err := td.GetIdempotencyKey("cached-idempotency-key")
	require.NoError(t, err)

	var previousJob models.Job
	err = json.Unmarshal(idempotencyRecord.Response, &previousJob)
	require.NoError(t, err)

	incomingJob := &models.Job{
		UserID:  idempotencyRecord.UserID,
		Type:    previousJob.Type,
		Payload: previousJob.Payload,
	}

	// Simulating duplicate key on insert
	repo.On("CreateIdempotencyKey", mock.Anything).Return(errors.New("duplicate key"))
	repo.On("GetIdempotencyKeyByKey", idempotencyRecord.IdempotencyKey).Return(idempotencyRecord, nil)

	res, err := service.CreateJob(incomingJob, idempotencyRecord.IdempotencyKey)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, previousJob.JobID, res.JobID)
	assert.Equal(t, previousJob.Status, res.Status)
}

func Test_GetJobByID_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	mockJob, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b918")
	require.NoError(t, err)

	repo.On("GetJobByID", mockJob.JobID).Return(mockJob, nil)

	res, err := service.GetJobByID(mockJob.JobID)
	require.NoError(t, err)
	assert.Equal(t, mockJob.JobID, res.JobID)
}

func Test_GetJobByID_NotFound(t *testing.T) {
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	jobID := "01a067a1-248e-779a-9fba-999999999999"
	repo.On("GetJobByID", jobID).Return(nil, gorm.ErrRecordNotFound)

	res, err := service.GetJobByID(jobID)
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, models.ErrJobNotFound))
}

func Test_ListJobs_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	repo.On("ListJobs").Return(td.Jobs, nil)

	jobs, err := service.ListJobs()
	require.NoError(t, err)
	assert.Len(t, jobs, len(td.Jobs))
}

func Test_UpdateJob_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	job, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b919")
	require.NoError(t, err)

	repo.On("UpdateJob", job).Return(nil)

	err = service.UpdateJob(job)
	assert.NoError(t, err)
}

func Test_DeleteJob_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	job, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b921")
	require.NoError(t, err)

	repo.On("DeleteJob", job.JobID).Return(nil)

	err = service.DeleteJob(job.JobID)
	assert.NoError(t, err)
}

func Test_CancelJob_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	job, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b922")
	require.NoError(t, err)

	repo.On("CancelJob", job.JobID).Return(nil)

	err = service.CancelJob(job.JobID)
	assert.NoError(t, err)
}

func Test_RetryJob_Success(t *testing.T) {
	td := testData.MustLoadTestData()
	repo := mocks.NewMockIJobRepo(t)
	service := NewJobService(repo, newDiscardLogger())

	job, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b921")
	require.NoError(t, err)

	repo.On("RetryJob", job.JobID).Return(nil)

	err = service.RetryJob(job.JobID)
	assert.NoError(t, err)
}
