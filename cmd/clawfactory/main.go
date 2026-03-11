// ClawFactory platform main service entry point.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/clawfactory/clawfactory/internal/api"
	"github.com/clawfactory/clawfactory/internal/config"
	"github.com/clawfactory/clawfactory/internal/event"
	"github.com/clawfactory/clawfactory/internal/memory"
	"github.com/clawfactory/clawfactory/internal/metrics"
	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/policy"
	"github.com/clawfactory/clawfactory/internal/registry"
	"github.com/clawfactory/clawfactory/internal/scheduler"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"github.com/clawfactory/clawfactory/internal/workflow"
)

// generateHeartbeatUUID generates a UUID v4 string for use in the heartbeat goroutine.
func generateHeartbeatUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Config holds the platform configuration.
type Config struct {
	Port           int      `json:"port"`
	DBPath         string   `json:"db_path"`
	DataDir        string   `json:"data_dir"`
	LogLevel       string   `json:"log_level"`
	APITokens      []string `json:"api_tokens"`
	MetricsEnabled bool     `json:"metrics_enabled"`
}

func loadConfig() Config {
	cfg := Config{
		Port: 8080, DBPath: "data/clawfactory.db",
		DataDir: "data", LogLevel: "info",
		MetricsEnabled: true,
		APITokens:      []string{"dev-token-001"},
	}

	configPath := os.Getenv("CLAWFACTORY_CONFIG")
	if configPath == "" {
		configPath = "configs/config.json"
	}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}

	if p := os.Getenv("CLAWFACTORY_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &cfg.Port)
	}
	if d := os.Getenv("CLAWFACTORY_DB_PATH"); d != "" {
		cfg.DBPath = d
	}
	if d := os.Getenv("CLAWFACTORY_DATA_DIR"); d != "" {
		cfg.DataDir = d
	}
	return cfg
}

func main() {
	cfg := loadConfig()

	// Initialize slog with JSON handler based on configured log level
	level := config.ParseSlogLevel(cfg.LogLevel)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Log observability configuration at startup
	slog.Info("observability config",
		"component", "main",
		"log_level", cfg.LogLevel,
		"metrics_enabled", cfg.MetricsEnabled,
	)

	// Ensure data directory exists
	os.MkdirAll(cfg.DataDir, 0755)

	// Initialize SQLite StateStore
	stateStore, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Initialize components
	queue := taskqueue.NewStoreBackedQueue(stateStore)
	mem := memory.NewFileSystemMemory(cfg.DataDir, stateStore)
	reg := registry.NewStoreRegistry(stateStore)

	policyPath := os.Getenv("CLAWFACTORY_POLICY_PATH")
	if policyPath == "" {
		policyPath = "configs/policy.json"
	}
	pe, err := policy.NewConfigPolicyEngine(policyPath, stateStore)
	if err != nil {
		slog.Error("failed to initialize policy engine", "error", err)
		os.Exit(1)
	}

	sched := scheduler.NewStoreScheduler(stateStore, queue)
	wfEngine := workflow.NewStoreWorkflowEngine(stateStore, queue)

	// Restore unfinished tasks
	unfinished, err := queue.RestoreUnfinished()
	if err != nil {
		slog.Warn("failed to restore unfinished tasks", "component", "main", "error", err)
	} else if len(unfinished) > 0 {
		slog.Info("restored unfinished tasks", "component", "main", "count", len(unfinished))
	}

	// Create MetricsCollector based on configuration
	var mc metrics.MetricsCollector
	if cfg.MetricsEnabled {
		mc = metrics.NewPrometheusCollector()
	} else {
		mc = metrics.NewNoopCollector()
	}

	// Create EventBus
	eventBus := event.NewStoreEventBus(stateStore)

	// Assemble HTTP service
	srv := &api.Server{
		Store:     stateStore,
		Registry:  reg,
		Scheduler: sched,
		Policy:    pe,
		Workflow:  wfEngine,
		Queue:     queue,
		Memory:    mem,
		Metrics:   mc,
		Events:    eventBus,
	}

	router := api.NewRouter(srv, cfg.APITokens, mc, cfg.MetricsEnabled)

	// Set up signal-based context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create http.Server for graceful shutdown support
	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start HTTP server in a goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "component", "main", "error", err)
			os.Exit(1)
		}
	}()

	// Start background heartbeat check goroutine
	var hbWg sync.WaitGroup
	hbWg.Add(1)
	go runHeartbeat(ctx, &hbWg, stateStore, reg, eventBus, mc)

	slog.Info("ClawFactory platform started", "component", "main", "address", addr)

	// Wait for shutdown signal
	<-ctx.Done()

	slog.Info("received shutdown signal, initiating graceful shutdown...", "component", "main")

	// Step 1: Shut down HTTP server with timeout
	slog.Info("shutting down HTTP server...", "component", "main")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP server shutdown error", "component", "main", "error", err)
	}
	slog.Info("HTTP server stopped", "component", "main")

	// Step 2: Wait for heartbeat goroutine to exit (already cancelled via ctx)
	slog.Info("waiting for heartbeat goroutine to stop...", "component", "main")
	hbWg.Wait()
	slog.Info("heartbeat goroutine stopped", "component", "main")

	// Step 3: Close database connection
	slog.Info("closing database connection...", "component", "main")
	if err := stateStore.Close(); err != nil {
		slog.Error("failed to close database", "component", "main", "error", err)
	}

	slog.Info("graceful shutdown complete", "component", "main")
}

// runHeartbeat runs the periodic heartbeat check loop. It exits when ctx is cancelled.
func runHeartbeat(ctx context.Context, wg *sync.WaitGroup, stateStore *store.SQLiteStore, reg registry.Registry, eventBus event.EventBus, mc metrics.MetricsCollector) {
	defer wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			marked, err := reg.CheckAndMarkOffline(90 * time.Second)
			if err != nil {
				slog.Error("heartbeat check failed", "component", "heartbeat", "error", err)
				continue
			}
			if len(marked) > 0 {
				slog.Info("marked agents as offline", "component", "heartbeat", "count", len(marked), "agent_ids", marked)
			}

			// Publish agent offline events and requeue tasks for offline agents
			for _, agentID := range marked {
				// Publish agent offline event
				if eventBus != nil {
					detail, _ := json.Marshal(map[string]string{})
					if err := eventBus.Publish(model.Event{
						EventID:    generateHeartbeatUUID(),
						EventType:  model.EventAgentOffline,
						EntityType: "agent",
						EntityID:   agentID,
						Detail:     string(detail),
						CreatedAt:  time.Now().UTC(),
					}); err != nil {
						slog.Warn("failed to publish event", "component", "heartbeat", "event_type", model.EventAgentOffline, "error", err)
					}
				}

				tasks, err := stateStore.GetTasksByAssignee(agentID)
				if err != nil {
					slog.Error("failed to get tasks for offline agent", "component", "heartbeat", "agent_id", agentID, "error", err)
					continue
				}
				for _, task := range tasks {
					if err := stateStore.RunInTransaction(func(tx *sql.Tx) error {
						return stateStore.RequeueTaskTx(tx, task.TaskID)
					}); err != nil {
						slog.Error("failed to requeue task for offline agent", "component", "heartbeat", "task_id", task.TaskID, "agent_id", agentID, "error", err)
						continue
					}

					// Publish task requeued event
					if eventBus != nil {
						detail, _ := json.Marshal(map[string]string{"reason": "agent_offline", "agent_id": agentID})
						if err := eventBus.Publish(model.Event{
							EventID:    generateHeartbeatUUID(),
							EventType:  model.EventTaskRequeued,
							EntityType: "task",
							EntityID:   task.TaskID,
							Detail:     string(detail),
							CreatedAt:  time.Now().UTC(),
						}); err != nil {
							slog.Warn("failed to publish event", "component", "heartbeat", "event_type", model.EventTaskRequeued, "error", err)
						}
					}
				}
				if len(tasks) > 0 {
					slog.Info("requeued tasks from offline agent", "component", "heartbeat", "count", len(tasks), "agent_id", agentID)
				}
			}

			// Update online agents gauge
			if mc != nil {
				agents, err := reg.ListAgents()
				if err == nil {
					onlineCount := 0
					for _, a := range agents {
						if a.Status == "online" {
							onlineCount++
						}
					}
					mc.SetAgentsOnline(float64(onlineCount))
				}
			}

			// Update queue depth gauge
			if mc != nil {
				unfinished, err := stateStore.ListUnfinishedTasks()
				if err == nil {
					pendingCount := 0
					for _, t := range unfinished {
						if t.Status == "pending" {
							pendingCount++
						}
					}
					mc.SetQueueDepth(float64(pendingCount))
				}
			}
		}
	}
}
