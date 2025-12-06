package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bretthaas/sandboxapi/internal/sandbox"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestExecuteJob(t *testing.T) {
	// Setup
	e := echo.New()
	reqBody := `{"language":"python", "code_snippet":"print(1)"}`
	req := httptest.NewRequest(http.MethodPost, "/execute", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockExec := sandbox.NewMockExecutor()
	h := NewHandler(mockExec)

	// Execute
	if assert.NoError(t, h.ExecuteJob(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var res sandbox.ExecutionResult
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, 0, res.ExitCode)
		assert.Contains(t, res.Stdout, "MOCK Python Output")
	}
}
