package api

import (
	"github.com/clawfactory/clawfactory/internal/event"
	"github.com/clawfactory/clawfactory/internal/memory"
	"github.com/clawfactory/clawfactory/internal/metrics"
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
	Metrics   metrics.MetricsCollector // v0.3: platform metrics
	Events    event.EventBus           // v0.3: event bus
}
