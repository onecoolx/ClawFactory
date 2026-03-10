# ClawFactory 快速入门

本文档将引导你从零开始搭建 ClawFactory 开发环境，启动平台服务，并运行一个完整的多智能体工作流。

## 前置条件

- Go 1.23.0 或更高版本
- Python 3.10 或更高版本（运行示例智能体）
- Git

## 第一步：获取源码

```bash
git clone https://github.com/clawfactory/clawfactory.git
cd clawfactory
```

## 第二步：安装 Go 依赖

```bash
# 中国大陆用户建议设置 Go 代理加速下载
export GOPROXY=https://goproxy.cn,direct

go mod tidy
```

## 第三步：编译

编译平台服务和 CLI 工具：

```bash
# 编译平台主服务
go build -o bin/clawfactory ./cmd/clawfactory

# 编译 CLI 工具
go build -o bin/claw ./cmd/claw
```

编译完成后，`bin/` 目录下会生成两个可执行文件：
- `clawfactory` — 平台主服务
- `claw` — 命令行管理工具

## 第四步：配置

### 平台配置

平台配置文件位于 `configs/config.json`：

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

| 字段 | 说明 | 默认值 |
|------|------|--------|
| port | HTTP 服务端口 | 8080 |
| db_path | SQLite 数据库文件路径 | data/clawfactory.db |
| data_dir | 数据目录（产出物存储等） | data |
| log_level | 日志级别 | info |
| api_tokens | 允许的 API Token 列表 | ["dev-token-001"] |

也可以通过环境变量覆盖配置：

```bash
export CLAWFACTORY_PORT=9090
export CLAWFACTORY_DB_PATH=/tmp/cf.db
export CLAWFACTORY_DATA_DIR=/tmp/cfdata
export CLAWFACTORY_CONFIG=/path/to/config.json
```

### 策略配置

策略配置文件位于 `configs/policy.json`，定义了角色权限、工具白名单和速率限制：

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

## 第五步：启动平台

```bash
./bin/clawfactory
```

看到以下输出说明启动成功：

```
ClawFactory 平台启动，监听端口 :8080
```

验证服务是否正常运行：

```bash
curl http://localhost:8080/health
```

应返回：

```json
{"status":"ok"}
```

## 第六步：使用 CLI 工具

### 提交工作流

项目自带一个软件开发工作流示例 `configs/software-dev-workflow.json`：

```bash
./bin/claw workflow submit configs/software-dev-workflow.json
```

输出：

```
工作流已提交: wf-inst-xxxxx
```

### 查询工作流状态

```bash
./bin/claw workflow status <workflow_id>
```

### 查看智能体列表

```bash
./bin/claw agent list
```

### 查看智能体日志

```bash
./bin/claw agent logs <agent_id>
```

### JSON 格式输出

所有 CLI 命令支持 `--output-json` 参数以 JSON 格式输出，方便脚本处理：

```bash
./bin/claw agent list --output-json
```

### 自定义服务地址和 Token

```bash
./bin/claw --url http://localhost:9090 --token my-secret-token workflow submit workflow.json
```

## 第七步：运行示例智能体

### 安装 Python 依赖

```bash
cd agents
pip install -r requirements.txt
```

如果需要使用 OpenAI API（示例智能体调用 LLM），设置 API Key：

```bash
export OPENAI_API_KEY=sk-your-api-key
```

### 启动智能体

在不同的终端窗口中分别启动各个智能体：

```bash
# 终端 1：需求分析智能体
python requirement_agent.py

# 终端 2：设计智能体
python design_agent.py

# 终端 3：编码智能体
python coding_agent.py

# 终端 4：测试智能体
python testing_agent.py
```

每个智能体启动后会自动：
1. 向平台注册自己
2. 开始定期发送心跳（每 30 秒）
3. 轮询拉取匹配自身能力的任务（每 5 秒）
4. 收到任务后自动执行并上报结果

## 完整流程演示

以下是一个完整的端到端流程：

```bash
# 1. 启动平台（终端 1）
./bin/clawfactory

# 2. 启动智能体（终端 2-5）
cd agents
python requirement_agent.py &
python design_agent.py &
python coding_agent.py &
python testing_agent.py &

# 3. 提交工作流（终端 6）
cd ..
./bin/claw workflow submit configs/software-dev-workflow.json
# 输出: 工作流已提交: wf-inst-xxxxx

# 4. 查看工作流状态
./bin/claw workflow status wf-inst-xxxxx

# 5. 查看产出物
./bin/claw workflow artifacts wf-inst-xxxxx

# 6. 查看智能体列表和日志
./bin/claw agent list
./bin/claw agent logs <agent_id>
```

工作流会按照 DAG 定义的顺序自动执行：
1. 需求分析智能体接收任务，分析用户需求，输出需求文档
2. 需求完成后，设计智能体接收任务，输出技术设计方案
3. 设计完成后，编码智能体接收任务，生成代码
4. 编码完成后，测试智能体接收任务，生成测试用例

## 运行测试

```bash
# 运行所有测试（包含属性测试）
go test ./...

# 运行特定包的测试
go test ./tests/internal/registry/...
go test ./tests/internal/scheduler/...

# 显示详细输出
go test -v ./tests/internal/workflow/...
```

## 下一步

- 阅读 [架构设计文档](architecture.md) 了解平台内部架构
- 阅读 [API 参考文档](api-reference.md) 了解所有接口详情
- 阅读 [用户手册](user-guide.md) 了解高级用法
- 阅读 [示例文档](examples.md) 了解更多使用场景
