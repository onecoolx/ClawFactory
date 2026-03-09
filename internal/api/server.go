package api

import (
	"github.com/clawfactory/clawfactory/internal/memory"
	"github.com/clawfactory/clawfactory/internal/policy"
	"github.com/clawfactory/clawfactory/internal/registry"
	"github.com/clawfactory/clawfactory/internal/scheduler"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"github.com/clawfactory/clawfactory/internal/workflow"
)

// Server holds all dependencies for the HTTP service.
type Server struct {
	Store     store.StateStore
	Registry  registry.Registry
	Scheduler scheduler.Scheduler
	Policy    policy.PolicyEngine
	Workflow  workflow.WorkflowEngine
	Queue     taskqueue.TaskQueue
	Memory    memory.SharedMemory
}
