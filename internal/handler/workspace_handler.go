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

type WorkspaceHandler struct {
	usecase   *usecases.WorkspaceUseCase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewWorkspaceHandler(uc *usecases.WorkspaceUseCase, v *validator.Validator, logger *slog.Logger) *WorkspaceHandler {
	return &WorkspaceHandler{usecase: uc, validator: v, logger: logger}
}

type createWorkspaceRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

type addMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid4"`
	Role   string `json:"role" validate:"required,oneof=admin member viewer"`
}

type workspaceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type memberResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

func toWorkspaceResponse(ws *domain.Workspace) workspaceResponse {
	return workspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Description: ws.Description,
		CreatedBy:   ws.CreatedBy,
		CreatedAt:   ws.CreatedAt,
	}
}

func toMemberResponse(m *domain.WorkspaceMember) memberResponse {
	return memberResponse{
		ID:          m.ID,
		WorkspaceID: m.WorkspaceID,
		UserID:      m.UserID,
		Role:        string(m.Role),
		JoinedAt:    m.JoinedAt,
	}
}

// Create Create workspace
//
// @Summary     Create workspace
// @Description Create new workspace. Author is assigned as admin.
// @Tags        workspaces
// @Accept      json
// @Produce     json
// @Param       request body     createWorkspaceRequest true "Workspace data"
// @Success     201     {object} workspaceResponse
// @Failure     400     {object} ProblemDetails
// @Failure     409     {object} ProblemDetails
// @Security    BearerAuth
// @Router      /workspaces [post]
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, h.logger, domain.ErrInvalidInput)
		return
	}

	if errs := h.validator.Validate(req); errs != nil {
		if err := RespondValidationError(w, errs); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"error writting validation response",
				"error",
				err,
				"path",
				r.URL.Path,
			)
		}
		return
	}

	userID := middlewares.GetUserID(r)
	if userID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	workspace, err := h.usecase.CreateWorkspace(r.Context(), req.Name, req.Description, userID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	if err := RespondJSON(w, http.StatusCreated, toWorkspaceResponse(workspace)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
}

// List List all workspaces from current user
//
// @Summary     List my workspaces
// @Description List all workspaces which user is member
// @Tags        workspaces
// @Produce     json
// @Success     200 {object} PaginatedResponse[workspaceResponse]
// @Security    BearerAuth
// @Router      /workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middlewares.GetUserID(r)
	if userID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	workspaces, err := h.usecase.ListUserWorkspaces(r.Context(), userID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	response := make([]workspaceResponse, len(workspaces))

	for i, workspace := range workspaces {
		response[i] = toWorkspaceResponse(workspace)
	}

	if err := RespondJSON(w, http.StatusOK, PaginatedResponse[workspaceResponse]{
		Data:    response,
		HasMore: false,
	}); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error getting JSON response to get tasks",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
}

// Get Get workspace by ID
//
// @Summary     Get workspace
// @Description Get a workspace. Only accesible by workspace's members
// @Tags        workspaces
// @Produce     json
// @Param       id  path     string true "Workspace ID"
// @Success     200 {object} workspaceResponse
// @Failure     403 {object} ProblemDetails
// @Failure     404 {object} ProblemDetails
// @Security    BearerAuth
// @Router      /workspaces/{id} [get]
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceId := r.PathValue("id")

	userID := middlewares.GetUserID(r)
	if userID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	workspace, err := h.usecase.GetWorkspace(r.Context(), workspaceId, userID)

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	if err := RespondJSON(w, http.StatusOK, toWorkspaceResponse(workspace)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
}

// AddMember Add member to workspace
//
// @Summary     Add member
// @Description Add member to workspace. Only admin can perform this.
// @Tags        workspaces
// @Accept      json
// @Produce     json
// @Param       id      path     string           true "workspace ID"
// @Param       request body     addMemberRequest true "Data of the member"
// @Success     201     {object} memberResponse
// @Failure     403     {object} ProblemDetails
// @Security    BearerAuth
// @Router      /workspaces/{id}/members [post]
func (h *WorkspaceHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	workspaceId := r.PathValue("id")

	var req addMemberRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, h.logger, domain.ErrInvalidInput)
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

	requesterID := middlewares.GetUserID(r)
	if requesterID == "" {
		RespondError(w, r, h.logger, domain.ErrUnauthorized)
		return
	}

	member, err := h.usecase.AddMember(r.Context(), workspaceId, requesterID, req.UserID, domain.WorkspaceRole(req.Role))

	if err != nil {
		RespondError(w, r, h.logger, err)
		return
	}

	if err := RespondJSON(w, http.StatusOK, toMemberResponse(member)); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"error writing JSON response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}

}
