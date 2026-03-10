#!/bin/sh
# ClawFactory agent launcher
# Usage: ./run_agent.sh <agent_script.py>
# Example: ./run_agent.sh requirement_agent.py

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <agent_script.py>"
    echo "Available agents:"
    echo "  requirement_agent.py  - Requirement analysis agent"
    echo "  design_agent.py       - Design agent"
    echo "  coding_agent.py       - Coding agent"
    echo "  testing_agent.py      - Testing agent"
    exit 1
fi

# LLM configuration
export OPENAI_API_KEY="${OPENAI_API_KEY:-}"
export OPENAI_BASE_URL="${OPENAI_BASE_URL:-}"
export MODEL_NAME="${MODEL_NAME:-gpt-4o-mini}"

# ClawFactory platform configuration
export CLAWFACTORY_TOKEN="${CLAWFACTORY_TOKEN:-dev-token-001}"

if [ -z "$OPENAI_API_KEY" ]; then
    echo "Warning: OPENAI_API_KEY is not set. LLM calls will fail."
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
python "$SCRIPT_DIR/$1"
