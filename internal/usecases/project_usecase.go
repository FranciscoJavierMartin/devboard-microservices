package usecases

import (
	"context"
	"fmt"

	"github.com/FranciscoJavierMartin/devboard/internal/domain"
)

type ProjectRepository interface {
	Create(ctx context.Context, project *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.Project, error)
}

type ProjectUseCase struct {
	projectRepo   ProjectRepository
	workspaceRepo WorkspaceRepository
}

func NewProjectUseCase(projectRepo ProjectRepository, workspaceRepo WorkspaceRepository) *ProjectUseCase {
	return &ProjectUseCase{
		projectRepo:   projectRepo,
		workspaceRepo: workspaceRepo,
	}
}

func (uc *ProjectUseCase) CreateProject(ctx context.Context, workspaceID, name, description, createdBy string) (*domain.Project, error) {
	_, err := uc.workspaceRepo.GetMember(ctx, workspaceID, createdBy)

	if err != nil {
		return nil, fmt.Errorf("workspace access denied: %w", domain.ErrForbidden)
	}

	project := domain.NewProject(workspaceID, name, description, createdBy)

	if err := uc.projectRepo.Create(ctx, project); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	return project, nil
}

func (uc *ProjectUseCase) GetProject(ctx context.Context, projectID, requesterID string) (*domain.Project, error) {
	project, err := uc.projectRepo.GetByID(ctx, projectID)

	if err != nil {
		return nil, fmt.Errorf("retrieving project: %w", err)
	}

	_, err = uc.workspaceRepo.GetMember(ctx, project.WorkspaceID, requesterID)

	if err != nil {
		return nil, fmt.Errorf("project access denied: %w", domain.ErrForbidden)
	}

	return project, nil
}

func (uc *ProjectUseCase) ListWorkspaceProjects(ctx context.Context, workspaceID, requesterID string) ([]*domain.Project, error) {
	_, err := uc.workspaceRepo.GetMember(ctx, workspaceID, requesterID)

	if err != nil {
		return nil, fmt.Errorf("workspace access denied: %w", domain.ErrForbidden)
	}

	projects, err := uc.projectRepo.ListByWorkspace(ctx, workspaceID)

	if err != nil {
		return nil, fmt.Errorf("retrieving projects: %w", err)
	}

	return projects, nil
}
