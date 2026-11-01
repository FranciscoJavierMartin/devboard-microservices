package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/FranciscoJavierMartin/devboard/internal/domain"
	"github.com/FranciscoJavierMartin/devboard/internal/middlewares"
	"github.com/FranciscoJavierMartin/devboard/internal/usecases"
	"github.com/FranciscoJavierMartin/devboard/internal/validator"
)

type CommentHandler struct {
	usecase   *usecases.CommentUseCase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewCommentHandler(uc *usecases.CommentUseCase, v *validator.Validator, logger *slog.Logger) *CommentHandler {
	return &CommentHandler{usecase: uc, validator: v, logger: logger}
}

type createCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=5000"`
}

type commentResponse struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	AuthorID  string    `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Create Add comment to a task
//
// @Summary     Create comment
// @Description Add comment to task
// @Tags        comments
// @Accept      json
// @Produce     json
// @Param       id      path     string               true "Task ID"
// @Param       request body     createCommentRequest true "Content"
// @Success     201     {object} commentResponse
// @Security    BearerAuth
// @Router      /tasks/{id}/comments [post]
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	authorID := middlewares.GetUserID(r)
	if authorID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	var req createCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, h.logger, domain.ErrInvalidInput)
		return
	}

	if errs := h.validator.Validate(req); errs != nil {
		if err := RespondValidationError(w, errs); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"error writing validation response",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
		}
		return
	}

	comment, err := h.usecase.CreateComment(r.Context(), taskID, authorID, req.Content)

	if err != nil {
		RespondError(w, r, h.logger, err)
	}

	if err := RespondJSON(w, http.StatusCreated, toCommentResponse(comment)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}

}

// ListByTask List all comment from a task
//
// @Summary     List comments
// @Description List all comments from a task
// @Tags        comments
// @Produce     json
// @Param       id  path     string true "task ID"
// @Success     200 {object} PaginatedResponse[commentResponse]
// @Security    BearerAuth
// @Router      /tasks/{id}/comments [get]
func (h *CommentHandler) ListByTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	comments, err := h.usecase.ListTaskComments(r.Context(), taskID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	response := make([]commentResponse, len(comments))

	for i, comment := range comments {
		response[i] = toCommentResponse(comment)
	}

	if err := RespondJSON(w, http.StatusOK, PaginatedResponse[commentResponse]{
		Data:    response,
		HasMore: false,
	}); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error getting JSON response to get comments",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}

}

func toCommentResponse(c *domain.Comment) commentResponse {
	return commentResponse{
		ID:        c.ID,
		TaskID:    c.TaskID,
		AuthorID:  c.AuthorID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
