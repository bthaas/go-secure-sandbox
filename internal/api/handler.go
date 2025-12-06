package api

import (
	"context"
	"net/http"
	"time"

	"github.com/bretthaas/sandboxapi/internal/sandbox"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	Executor sandbox.Executor
}

func NewHandler(e sandbox.Executor) *Handler {
	return &Handler{Executor: e}
}

func (h *Handler) ExecuteJob(c echo.Context) error {
	var req sandbox.ExecutionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload").SetInternal(err)
	}

	// Basic validation
	if req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code_snippet is required")
	}
	if req.Language == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "language is required")
	}

	// Default timeout
	timeoutVal := req.TimeoutMs
	if timeoutVal <= 0 {
		timeoutVal = 5000 // 5s default
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Duration(timeoutVal)*time.Millisecond)
	defer cancel()

	result, err := h.Executor.Execute(ctx, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Execution failed").SetInternal(err)
	}

	return c.JSON(http.StatusOK, result)
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/execute", h.ExecuteJob)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
}
