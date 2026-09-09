package repositories

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	dialector := postgres.New(postgres.Config{
		Conn:                 mockDB,
		DriverName:           "postgres",
		PreferSimpleProtocol: true,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	return gormDB, mock, mockDB
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWorkerRepository_SaveJobResult_NewRecord(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-123"
	resultData := "test output result"

	// 1. SELECT query finds no existing record
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "job_results" WHERE job_id = $1 AND "job_results"."deleted_at" IS NULL ORDER BY "job_results"."id" LIMIT $2`)).
		WithArgs(jobID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// 2. INSERT new record
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "job_results"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	err := repo.SaveJobResult(context.Background(), jobID, resultData)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkerRepository_SaveJobResult_ExistingRecord(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-123"
	resultData := "updated output result"

	// 1. SELECT query finds existing record
	columns := []string{"id", "result_id", "job_id", "status", "result_data", "created_at", "updated_at", "deleted_at"}
	rows := sqlmock.NewRows(columns).
		AddRow(uint(1), "res-1", jobID, "pending", []byte("old output"), time.Now(), time.Now(), nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "job_results" WHERE job_id = $1 AND "job_results"."deleted_at" IS NULL ORDER BY "job_results"."id" LIMIT $2`)).
		WithArgs(jobID, 1).
		WillReturnRows(rows)

	// 2. UPDATE existing record
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "job_results"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.SaveJobResult(context.Background(), jobID, resultData)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkerRepository_GetJobResult_Success(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-123"
	expectedResult := "computation completed"

	columns := []string{"result_id", "job_id", "result_data", "status", "created_at", "updated_at", "deleted_at"}
	rows := sqlmock.NewRows(columns).
		AddRow("res-1", jobID, []byte(expectedResult), "completed", time.Now(), time.Now(), nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "job_results" WHERE job_id = $1 AND "job_results"."deleted_at" IS NULL ORDER BY "job_results"."id" LIMIT $2`)).
		WithArgs(jobID, 1).
		WillReturnRows(rows)

	res, err := repo.GetJobResult(context.Background(), jobID)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, res)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkerRepository_GetJobResult_NotFound(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-non-existent"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "job_results" WHERE job_id = $1 AND "job_results"."deleted_at" IS NULL ORDER BY "job_results"."id" LIMIT $2`)).
		WithArgs(jobID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	res, err := repo.GetJobResult(context.Background(), jobID)
	assert.Error(t, err)
	assert.Empty(t, res)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkerRepository_DeleteJobResult(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-123"

	// Since JobResult embeds gorm.Model, Delete produces soft delete (UPDATE ... SET deleted_at = ...)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "job_results" SET "deleted_at"=$1 WHERE job_id = $2 AND "job_results"."deleted_at" IS NULL`)).
		WithArgs(sqlmock.AnyArg(), jobID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.DeleteJobResult(context.Background(), jobID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkerRepository_UpdateJobStatus(t *testing.T) {
	gormDB, mock, rawDB := setupTestDB(t)
	defer rawDB.Close()

	repo := NewWorkerRepository(gormDB, newTestLogger())
	jobID := "job-123"

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "jobs" SET`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.UpdateJobStatus(context.Background(), jobID, models.StatusCompleted)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
