package services

import (
	"log/slog"
	"github.com/gedyzed/JobFlow/JobService/repositories"
	"github.com/gedyzed/JobFlow/JobService/models"
)

type IJobService interface {
	CreateJob(job *models.Job) (*models.Job, error)
	GetJobByID(id string) (*models.Job, error)
	ListJobs() ([]*models.Job, error)
	UpdateJob(job *models.Job) error
	DeleteJob(id string) error
	CancelJob(id string) error
	RetryJob(id string) error
}

type JobService struct {
	repo repositories.IJobRepo
	logger *slog.Logger
}

func NewJobService(repo repositories.IJobRepo, logger *slog.Logger) IJobService {
	return &JobService{
		repo:   repo,
		logger: logger,
	}
}

func (s *JobService) CreateJob(job *models.Job) (*models.Job, error) {
	return s.repo.CreateJob(job)
}

func (s *JobService) GetJobByID(id string) (*models.Job, error) {
	return s.repo.GetJobByID(id)
}

func (s *JobService) ListJobs() ([]*models.Job, error) {
	return s.repo.ListJobs()
}

func (s *JobService) UpdateJob(job *models.Job) error {
	return s.repo.UpdateJob(job)
}

func (s *JobService) DeleteJob(id string) error {		
	return s.repo.DeleteJob(id)
}

func (s *JobService) CancelJob(id string) error {
	return s.repo.CancelJob(id)
}

func (s *JobService) RetryJob(id string) error {
	return s.repo.RetryJob(id)
}

