package testData

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTestData(t *testing.T) {
	td, err := LoadTestData()
	require.NoError(t, err)
	require.NotNil(t, td)

	assert.NotEmpty(t, td.Jobs)
	assert.NotEmpty(t, td.IdempotencyKeys)
	assert.NotEmpty(t, td.OutboxEvents)

	for _, job := range td.Jobs {
		assert.NoError(t, job.ValidateWithError())
	}
}

func TestMustLoadTestData(t *testing.T) {
	assert.NotPanics(t, func() {
		td := MustLoadTestData()
		assert.NotNil(t, td)
	})
}

func TestTestData_GetJobByID(t *testing.T) {
	td := MustLoadTestData()

	targetID := "01a067a1-248e-779a-9fba-a101f0f2b918"
	job, err := td.GetJobByID(targetID)
	require.NoError(t, err)
	assert.Equal(t, targetID, job.JobID)
	assert.Equal(t, models.JobTypeSendEmail, job.Type)
	assert.Equal(t, models.StatusPending, job.Status)

	_, err = td.GetJobByID("non-existent-id")
	assert.Error(t, err)
}

func TestTestData_GetJobByType(t *testing.T) {
	td := MustLoadTestData()

	job, err := td.GetJobByType(models.JobTypeGenerateReport)
	require.NoError(t, err)
	assert.Equal(t, models.JobTypeGenerateReport, job.Type)

	_, err = td.GetJobByType("UNKNOWN_TYPE")
	assert.Error(t, err)
}

func TestTestData_GetJobByStatus(t *testing.T) {
	td := MustLoadTestData()

	job, err := td.GetJobByStatus(models.StatusRunning)
	require.NoError(t, err)
	assert.Equal(t, models.StatusRunning, job.Status)

	_, err = td.GetJobByStatus("unknown_status")
	assert.Error(t, err)
}

func TestTestData_GetJobsByStatus(t *testing.T) {
	td := MustLoadTestData()

	pendingJobs := td.GetJobsByStatus(models.StatusPending)
	assert.NotEmpty(t, pendingJobs)
	for _, j := range pendingJobs {
		assert.Equal(t, models.StatusPending, j.Status)
	}

	none := td.GetJobsByStatus("non_existent_status")
	assert.Empty(t, none)
}

func TestTestData_GetJobsByType(t *testing.T) {
	td := MustLoadTestData()

	emailJobs := td.GetJobsByType(models.JobTypeSendEmail)
	assert.NotEmpty(t, emailJobs)
	for _, j := range emailJobs {
		assert.Equal(t, models.JobTypeSendEmail, j.Type)
	}

	none := td.GetJobsByType("NON_EXISTENT")
	assert.Empty(t, none)
}

func TestTestData_GetIdempotencyKey(t *testing.T) {
	td := MustLoadTestData()

	key, err := td.GetIdempotencyKey("unique-idempotency-key")
	require.NoError(t, err)
	assert.Equal(t, "unique-idempotency-key", key.IdempotencyKey)
	assert.Equal(t, "pending", key.Status)

	_, err = td.GetIdempotencyKey("missing-key")
	assert.Error(t, err)
}

func TestTestData_GetOutboxByJobID(t *testing.T) {
	td := MustLoadTestData()

	targetJobID := "01a067a1-248e-779a-9fba-a101f0f2b918"
	outbox, err := td.GetOutboxByJobID(targetJobID)
	require.NoError(t, err)
	assert.Equal(t, targetJobID, outbox.JobID)
	assert.Equal(t, "job.created", outbox.EventType)

	_, err = td.GetOutboxByJobID("missing-job-id")
	assert.Error(t, err)
}

func TestCloneJob(t *testing.T) {
	td := MustLoadTestData()
	orig, err := td.GetJobByID("01a067a1-248e-779a-9fba-a101f0f2b918")
	require.NoError(t, err)

	clone, err := CloneJob(orig)
	require.NoError(t, err)
	assert.Equal(t, orig.JobID, clone.JobID)

	clone.Status = models.StatusCompleted
	assert.NotEqual(t, orig.Status, clone.Status)
	assert.Equal(t, models.StatusPending, orig.Status)

	nilClone, err := CloneJob(nil)
	assert.NoError(t, err)
	assert.Nil(t, nilClone)
}

func TestLoadTestDataFromFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "custom.json")

	validJSON := `{"jobs":[{"job_id":"01a067a1-248e-779a-9fba-a101f0f2b999","user_id":"u1","type":"SEND_EMAIL","status":"pending","payload":{"to":["a@b.com"]}}]}`
	err := os.WriteFile(tempFile, []byte(validJSON), 0644)
	require.NoError(t, err)

	td, err := LoadTestDataFromFile(tempFile)
	require.NoError(t, err)
	assert.Len(t, td.Jobs, 1)
	assert.Equal(t, "01a067a1-248e-779a-9fba-a101f0f2b999", td.Jobs[0].JobID)

	_, err = LoadTestDataFromFile("non-existent-file.json")
	assert.Error(t, err)
}

func TestLoadTestDataFromBytes_Invalid(t *testing.T) {
	_, err := LoadTestDataFromBytes([]byte("invalid json"))
	assert.Error(t, err)
}
