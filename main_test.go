package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *TaskStore) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	enforcer, err := casbin.NewEnforcer("authz/model.conf", "authz/policy.csv")
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}

	store := NewTaskStore()
	server := NewServer(store, enforcer)
	return server.Router(), store
}

func TestACLAllowsAliceToCreateTask(t *testing.T) {
	router, _ := setupTestRouter(t)

	res := performRequest(router, http.MethodPost, "/tasks", "alice", `{"title":"Comprar leite"}`)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}

	var task Task
	if err := json.Unmarshal(res.Body.Bytes(), &task); err != nil {
		t.Fatalf("failed to decode task: %v", err)
	}
	if task.Owner != "alice" {
		t.Fatalf("expected alice as owner, got %q", task.Owner)
	}
}

func TestACLDeniesBobCreatingTask(t *testing.T) {
	router, _ := setupTestRouter(t)

	res := performRequest(router, http.MethodPost, "/tasks", "bob", `{"title":"Comprar leite"}`)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestRBACAllowsUserToListTasks(t *testing.T) {
	router, store := setupTestRouter(t)
	store.Create(Task{Title: "Tarefa existente", Owner: "alice"})

	res := performRequest(router, http.MethodGet, "/tasks", "bob", "")

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
}

func TestRBACAllowsAdminToDeleteAnyTask(t *testing.T) {
	router, store := setupTestRouter(t)
	task := store.Create(Task{Title: "Tarefa da Alice", Owner: "alice"})

	res := performRequest(router, http.MethodDelete, "/tasks/"+itoa(task.ID), "admin", "")

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, res.Code, res.Body.String())
	}
}

func TestABACAllowsOwnerToUpdateTask(t *testing.T) {
	router, store := setupTestRouter(t)
	task := store.Create(Task{Title: "Rascunho", Owner: "alice"})

	res := performRequest(router, http.MethodPut, "/tasks/"+itoa(task.ID), "alice", `{"title":"Finalizada","done":true}`)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
}

func TestABACDeniesNonOwnerUpdatingTask(t *testing.T) {
	router, store := setupTestRouter(t)
	task := store.Create(Task{Title: "Tarefa da Alice", Owner: "alice"})

	res := performRequest(router, http.MethodPut, "/tasks/"+itoa(task.ID), "bob", `{"title":"Alterada"}`)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
}

func TestABACDeniesNonOwnerDeletingTask(t *testing.T) {
	router, store := setupTestRouter(t)
	task := store.Create(Task{Title: "Tarefa da Alice", Owner: "alice"})

	res := performRequest(router, http.MethodDelete, "/tasks/"+itoa(task.ID), "bob", "")

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
}

func TestAuthorizationRequiresUserHeader(t *testing.T) {
	router, _ := setupTestRouter(t)

	res := performRequest(router, http.MethodGet, "/tasks", "", "")

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func performRequest(router http.Handler, method, path, user, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.Header.Set("X-User", user)
	}

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func itoa(id int) string {
	return strconv.Itoa(id)
}
