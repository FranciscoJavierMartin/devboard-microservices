package usecases

import (
	"context"
	"fmt"

	"github.com/FranciscoJavierMartin/devboard/internal/domain"
)

type WorkspaceRepository interface {
	CreateWorkspace(ctx context.Context, ws *domain.Workspace) error
	GetWorkspaceByID(ctx context.Context, id string) (*domain.Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (*domain.Workspace, error)
	ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	AddMember(ctx context.Context, member *domain.WorkspaceMember) error
	GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error)
	ListMembers(ctx context.Context, workspaceID string) ([]*domain.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID string) error
}

type WorkspaceUseCase struct {
	repo WorkspaceRepository
}

func NewWorkspaceUseCase(repo WorkspaceRepository) *WorkspaceUseCase {
	return &WorkspaceUseCase{
		repo: repo,
	}
}

func (uc *WorkspaceUseCase) CreateWorkspace(ctx context.Context, name, description, createdBy string) (*domain.Workspace, error) {
	slug := domain.GenerateSlug(name)
	existing, err := uc.repo.GetWorkspaceBySlug(ctx, slug)

	if err == nil && existing != nil {
		return nil, fmt.Errorf("workspace with slug %q: %w", slug, domain.ErrAlreadyExists)
	}

	ws := domain.NewWorkspace(name, description, createdBy)

	if err := uc.repo.CreateWorkspace(ctx, ws); err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}

	member := domain.NewWorkspaceMember(ws.ID, createdBy, domain.WorkspaceRoleAdmin)

	if err := uc.repo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("adding author as admin: %w", err)
	}

	return ws, nil
}

func (uc *WorkspaceUseCase) GetWorkspace(ctx context.Context, workspaceID, requesterID string) (*domain.Workspace, error) {
	if err := uc.requireMember(ctx, workspaceID, requesterID); err != nil {
		return nil, err
	}

	ws, err := uc.repo.GetWorkspaceByID(ctx, workspaceID)

	if err != nil {
		return nil, fmt.Errorf("retrieving workspace: %w", err)
	}

	return ws, nil
}

func (uc *WorkspaceUseCase) ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	workspaces, err := uc.repo.ListWorkspacesByUser(ctx, userID)

	if err != nil {
		return nil, fmt.Errorf("retrieving workspaces from user %s: %w", userID, err)
	}

	return workspaces, nil
}

func (uc *WorkspaceUseCase) AddMember(ctx context.Context, workspaceID, requesterID, newUserID string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error) {
	if err := uc.requireAdmin(ctx, workspaceID, requesterID); err != nil {
		return nil, err
	}

	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role %q: %w", role, domain.ErrInvalidInput)
	}

	member := domain.NewWorkspaceMember(workspaceID, newUserID, role)

	if err := uc.repo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("adding member: %w", err)
	}

	return member, nil
}

func (uc *WorkspaceUseCase) ListMembers(ctx context.Context, workspaceID, requesterID string) ([]*domain.WorkspaceMember, error) {
	if err := uc.requireMember(ctx, workspaceID, requesterID); err != nil {
		return nil, err
	}

	return uc.repo.ListMembers(ctx, workspaceID)
}

func (uc *WorkspaceUseCase) RemoveMember(ctx context.Context, workspaceID, requesterID, targetUserID string) error {

}

func (uc *WorkspaceUseCase) requireMember(ctx context.Context, workspaceID, userID string) error {
	_, err := uc.repo.GetMember(ctx, workspaceID, userID)

	if err != nil {
		return fmt.Errorf("workspace access denied: %w", domain.ErrForbidden)
	}

	return nil
}

func (uc *WorkspaceUseCase) requireAdmin(ctx context.Context, workspaceID, userID string) error {
	member, err := uc.repo.GetMember(ctx, workspaceID, userID)

	if err != nil {
		return fmt.Errorf("workspace access denied: %w", domain.ErrForbidden)
	}

	if member.Role != domain.WorkspaceRoleAdmin {
		return fmt.Errorf("admin role is required: %w", domain.ErrForbidden)
	}

	return nil
}
