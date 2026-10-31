package usecases

import (
	"context"
	"fmt"

	"github.com/FranciscoJavierMartin/devboard/internal/domain"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *domain.Comment) error
	GetByID(ctx context.Context, id string) (*domain.Comment, error)
	ListByTask(ctx context.Context, taskID string) ([]*domain.Comment, error)
	Delete(ctx context.Context, id string) error
}

type CommentUseCase struct {
	commentRepo CommentRepository
	taskQuery   TaskQueryRepository
}

func NewCommentUseCase(commentRepo CommentRepository, taskQuery TaskQueryRepository) *CommentUseCase {
	return &CommentUseCase{
		commentRepo: commentRepo,
		taskQuery:   taskQuery,
	}
}

func (uc *CommentUseCase) CreateComment(ctx context.Context, taskID, authorID, content string) (*domain.Comment, error) {
	_, err := uc.taskQuery.GetByID(ctx, taskID)

	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	comment, err := domain.NewComment(taskID, authorID, content)

	if err != nil {
		return nil, fmt.Errorf("creating comment: %w", err)
	}

	if err := uc.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("saving comment: %w", err)
	}

	return comment, nil
}

func (uc *CommentUseCase) ListTaskComments(ctx context.Context, taskID string) ([]*domain.Comment, error) {
	comments, err := uc.commentRepo.ListByTask(ctx, taskID)

	if err != nil {
		return nil, fmt.Errorf("retrieving comments: %w", err)
	}

	return comments, nil
}

func (uc *CommentUseCase) DeleteComment(ctx context.Context, commentID, requesterID string) error {
	comment, err := uc.commentRepo.GetByID(ctx, commentID)

	if err != nil {
		return fmt.Errorf("retrieving comment: %w", err)
	}

	if comment.AuthorID != requesterID {
		return fmt.Errorf("comments can only be removed by author: %w", domain.ErrForbidden)
	}

	return uc.commentRepo.Delete(ctx, commentID)
}
