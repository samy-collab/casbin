package main

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Done        bool   `json:"done"`
}

type TaskStore struct {
	mu     sync.RWMutex
	nextID int
	tasks  map[int]Task
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		nextID: 1,
		tasks:  make(map[int]Task),
	}
}

func (s *TaskStore) Create(task Task) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = s.nextID
	s.nextID++
	s.tasks[task.ID] = task
	return task
}

func (s *TaskStore) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (s *TaskStore) Get(id int) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	return task, ok
}

func (s *TaskStore) Update(id int, input Task) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, false
	}

	task.Title = input.Title
	task.Description = input.Description
	task.Done = input.Done
	s.tasks[id] = task
	return task, true
}

func (s *TaskStore) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return false
	}
	delete(s.tasks, id)
	return true
}

type Server struct {
	store    *TaskStore
	enforcer *casbin.Enforcer
}

func NewServer(store *TaskStore, enforcer *casbin.Enforcer) *Server {
	return &Server{
		store:    store,
		enforcer: enforcer,
	}
}

func (s *Server) Router() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	tasks := router.Group("/tasks")
	tasks.Use(s.Authorize())
	tasks.POST("", s.createTask)
	tasks.GET("", s.listTasks)
	tasks.GET("/:id", s.getTask)
	tasks.PUT("/:id", s.updateTask)
	tasks.DELETE("/:id", s.deleteTask)

	return router
}

func (s *Server) Authorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.GetHeader("X-User")
		if user == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing X-User header"})
			return
		}

		owner := ""
		if idText := c.Param("id"); idText != "" {
			id, err := strconv.Atoi(idText)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
				return
			}

			task, ok := s.store.Get(id)
			if !ok {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "task not found"})
				return
			}
			owner = task.Owner
		}

		allowed, err := s.enforcer.Enforce(user, c.FullPath(), c.Request.Method, owner)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.Next()
	}
}

func (s *Server) createTask(c *gin.Context) {
	user := c.GetHeader("X-User")

	var input Task
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.Owner = user
	task := s.store.Create(input)
	c.JSON(http.StatusCreated, task)
}

func (s *Server) listTasks(c *gin.Context) {
	c.JSON(http.StatusOK, s.store.List())
}

func (s *Server) getTask(c *gin.Context) {
	id, err := taskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, ok := s.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) updateTask(c *gin.Context) {
	id, err := taskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var input Task
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, ok := s.store.Update(id, input)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) deleteTask(c *gin.Context) {
	id, err := taskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !s.store.Delete(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func taskID(c *gin.Context) (int, error) {
	idText := c.Param("id")
	if idText == "" {
		return 0, errors.New("missing task id")
	}
	return strconv.Atoi(idText)
}

func main() {
	enforcer, err := casbin.NewEnforcer("authz/model.conf", "authz/policy.csv")
	if err != nil {
		panic(err)
	}

	server := NewServer(NewTaskStore(), enforcer)
	if err := server.Router().Run(":8080"); err != nil {
		panic(err)
	}
}
