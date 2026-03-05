// ClawFactory 平台主服务入口
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/clawfactory/clawfactory/internal/api"
	"github.com/clawfactory/clawfactory/internal/memory"
	"github.com/clawfactory/clawfactory/internal/policy"
	"github.com/clawfactory/clawfactory/internal/registry"
	"github.com/clawfactory/clawfactory/internal/scheduler"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"github.com/clawfactory/clawfactory/internal/workflow"
)

type Config struct {
	Port      int      `json:"port"`
	DBPath    string   `json:"db_path"`
	DataDir   string   `json:"data_dir"`
	LogLevel  string   `json:"log_level"`
	APITokens []string `json:"api_tokens"`
}

func loadConfig() Config {
	cfg := Config{
		Port: 8080, DBPath: "data/clawfactory.db",
		DataDir: "data", LogLevel: "info",
		APITokens: []string{"dev-token-001"},
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

	// 确保数据目录存在
	os.MkdirAll(cfg.DataDir, 0755)

	// 初始化 SQLite StateStore
	stateStore, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer stateStore.Close()

	// 初始化各组件
	queue := taskqueue.NewStoreBackedQueue(stateStore)
	mem := memory.NewFileSystemMemory(cfg.DataDir, stateStore)
	reg := registry.NewStoreRegistry(stateStore)

	policyPath := os.Getenv("CLAWFACTORY_POLICY_PATH")
	if policyPath == "" {
		policyPath = "configs/policy.json"
	}
	pe, err := policy.NewConfigPolicyEngine(policyPath, stateStore)
	if err != nil {
		log.Fatalf("初始化策略引擎失败: %v", err)
	}

	sched := scheduler.NewStoreScheduler(stateStore, queue)
	wfEngine := workflow.NewStoreWorkflowEngine(stateStore, queue)

	// 恢复未完成任务
	unfinished, err := queue.RestoreUnfinished()
	if err != nil {
		log.Printf("恢复未完成任务失败: %v", err)
	} else if len(unfinished) > 0 {
		log.Printf("恢复了 %d 个未完成任务", len(unfinished))
	}

	// 组装 HTTP 服务
	srv := &api.Server{
		Store:     stateStore,
		Registry:  reg,
		Scheduler: sched,
		Policy:    pe,
		Workflow:  wfEngine,
		Queue:     queue,
		Memory:    mem,
	}

	router := api.NewRouter(srv, cfg.APITokens)

	// 启动后台心跳检查 goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			marked, err := reg.CheckAndMarkOffline(90 * time.Second)
			if err != nil {
				log.Printf("心跳检查失败: %v", err)
			} else if len(marked) > 0 {
				log.Printf("标记 %d 个智能体为 offline: %v", len(marked), marked)
			}
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("ClawFactory 平台启动，监听端口 %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
