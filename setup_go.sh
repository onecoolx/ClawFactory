#!/bin/bash
set -e

# 检查 Go 是否已安装
if command -v go &> /dev/null; then
    echo "GO_FOUND: $(go version)"
    exit 0
fi

# 检查常见路径
for p in /usr/local/go/bin/go /usr/lib/go/bin/go /snap/bin/go $HOME/go/bin/go; do
    if [ -x "$p" ]; then
        echo "GO_FOUND: $($p version)"
        exit 0
    fi
done

echo "GO_NOT_FOUND"

# 安装 Go 1.23
echo "Installing Go 1.23..."
wget -q https://go.dev/dl/go1.23.4.linux-amd64.tar.gz -O /tmp/go.tar.gz
sudo tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo "GO_INSTALLED: $(go version)"
