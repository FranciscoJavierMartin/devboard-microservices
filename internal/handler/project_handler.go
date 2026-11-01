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

type ProjectHandler struct {
	usecase   *usecases.ProjectUseCase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewProjectHandler(uc *usecases.ProjectUseCase, v *validator.Validator, logger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{usecase: uc, validator: v, logger: logger}
}

type createProjectRequest struct {
	WorkspaceID string `json:"workspace_id" validate:"required,uuid4"`
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

type projectResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Create Create new project
//
// @Summary     Create project
// @Description Create new project inside workspace
// @Tags        projects
// @Accept      json
// @Produce     json
// @Param       request body     createProjectRequest true "Project data"
// @Success     201     {object} projectResponse
// @Security    BearerAuth
// @Router      /projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest

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

	userID := middlewares.GetUserID(r)
	if userID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	project, err := h.usecase.CreateProject(r.Context(), req.WorkspaceID, req.Name, req.Description, userID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	if err := RespondJSON(w, http.StatusCreated, toProjectResponse(project)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
}

// Get Get project by ID
//
// @Summary     Get project
// @Description Retrieve project by ID
// @Tags        projects
// @Produce     json
// @Param       id  path     string true "Project ID"
// @Success     200 {object} projectResponse
// @Security    BearerAuth
// @Router      /projects/{id} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	userID := middlewares.GetUserID(r)
	if userID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	project, err := h.usecase.GetProject(r.Context(), id, userID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	if err := RespondJSON(w, http.StatusOK, toProjectResponse(project)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
}

func toProjectResponse(p *domain.Project) projectResponse {
	return projectResponse{
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		Name:        p.Name,
		Description: p.Description,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt,
	}
}
