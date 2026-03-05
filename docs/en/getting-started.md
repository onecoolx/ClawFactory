# ClawFactory Getting Started

This guide walks you through setting up the ClawFactory development environment from scratch, starting the platform service, and running a complete multi-agent workflow.

## Prerequisites

- Go 1.23.0 or later
- Python 3.10 or later (for running example agents)
- Git

## Step 1: Get the Source Code

```bash
git clone https://github.com/clawfactory/clawfactory.git
cd clawfactory
```

## Step 2: Install Go Dependencies

```bash
go mod tidy
```

## Step 3: Build

Compile the platform service and CLI tool:

```bash
# Build the platform service
go build -o bin/clawfactory ./cmd/clawfactory

# Build the CLI tool
go build -o bin/claw ./cmd/claw
```

After building, the `bin/` directory will contain two executables:
- `clawfactory` — Platform main service
- `claw` — Command-line management tool

## Step 4: Configuration

### Platform Configuration

The platform configuration file is located at `configs/config.json`:

```json
{
    "port": 8080,
    "db_path": "data/clawfactory.db",
    "data_dir": "data",
    "log_level": "info",
    "api_tokens": [
        "dev-token-001"
    ]
}
```

| Field | Description | Default |
|-------|-------------|---------|
| port | HTTP service port | 8080 |
| db_path | SQLite database file path | data/clawfactory.db |
| data_dir | Data directory (artifact storage, etc.) | data |
| log_level | Log level | info |
| api_tokens | Allowed API token list | ["dev-token-001"] |

Configuration can be overridden with environment variables:

```bash
export CLAWFACTORY_PORT=9090
export CLAWFACTORY_DB_PATH=/tmp/cf.db
export CLAWFACTORY_DATA_DIR=/tmp/cfdata
export CLAWFACTORY_CONFIG=/path/to/config.json
```

### Policy Configuration

The policy configuration file at `configs/policy.json` defines role permissions, tool whitelists, and rate limits:

```json
{
    "max_retries": 3,
    "heartbeat_interval_seconds": 30,
    "heartbeat_timeout_multiplier": 3,
    "roles": {
        "developer_agent": {
            "permissions": [
                {"resource": "shared_memory:*", "actions": ["read", "write"]},
                {"resource": "task:*", "actions": ["read", "update"]}
            ]
        }
    },
    "tool_whitelist": {
        "developer_agent": {
            "allowed_tools": ["llm_api", "file_write", "file_read"],
            "rate_limit_per_minute": 60
        }
    }
}
```

## Step 5: Start the Platform

```bash
./bin/clawfactory
```

You should see:

```
ClawFactory 平台启动，监听端口 :8080
```

Verify the service is running:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{"status":"ok"}
```

## Step 6: Use the CLI Tool

### Submit a Workflow

The project includes a software development workflow example at `configs/software-dev-workflow.json`:

```bash
./bin/claw workflow submit configs/software-dev-workflow.json
```

### Query Workflow Status

```bash
./bin/claw workflow status <workflow_id>
```

### List Agents

```bash
./bin/claw agent list
```

### View Agent Logs

```bash
./bin/claw agent logs <agent_id>
```

### JSON Output

All CLI commands support `--output-json` for JSON-formatted output:

```bash
./bin/claw agent list --output-json
```

### Custom Server Address and Token

```bash
./bin/claw --url http://localhost:9090 --token my-secret-token workflow submit workflow.json
```

## Step 7: Run Example Agents

### Install Python Dependencies

```bash
cd agents
pip install -r requirements.txt
```

If using the OpenAI API (example agents call LLM), set the API key:

```bash
export OPENAI_API_KEY=sk-your-api-key
```

### Start Agents

Start each agent in a separate terminal:

```bash
# Terminal 1: Requirement analysis agent
python requirement_agent.py

# Terminal 2: Design agent
python design_agent.py

# Terminal 3: Coding agent
python coding_agent.py

# Terminal 4: Testing agent
python testing_agent.py
```

Each agent will automatically:
1. Register with the platform
2. Start sending periodic heartbeats (every 30 seconds)
3. Poll for matching tasks (every 5 seconds)
4. Execute tasks and report results when assigned

## Full End-to-End Demo

```bash
# 1. Start the platform (Terminal 1)
./bin/clawfactory

# 2. Start agents (Terminals 2-5)
cd agents
python requirement_agent.py &
python design_agent.py &
python coding_agent.py &
python testing_agent.py &

# 3. Submit workflow (Terminal 6)
cd ..
./bin/claw workflow submit configs/software-dev-workflow.json

# 4. Check workflow status
./bin/claw workflow status <workflow_id>

# 5. View artifacts
./bin/claw workflow artifacts <workflow_id>

# 6. View agents and logs
./bin/claw agent list
./bin/claw agent logs <agent_id>
```

The workflow executes in DAG order:
1. Requirement agent receives task, analyzes requirements, outputs requirement document
2. After requirements complete, design agent receives task, outputs technical design
3. After design completes, coding agent receives task, generates code
4. After coding completes, testing agent receives task, generates test cases

## Running Tests

```bash
# Run all tests (including property-based tests)
go test ./...

# Run tests for a specific package
go test ./internal/registry/...
go test ./internal/scheduler/...

# Verbose output
go test -v ./internal/workflow/...
```

## Next Steps

- Read the [Architecture Document](architecture.md) to understand the platform internals
- Read the [API Reference](api-reference.md) for complete endpoint documentation
- Read the [User Guide](user-guide.md) for advanced usage
- Read the [Examples](examples.md) for more use cases
