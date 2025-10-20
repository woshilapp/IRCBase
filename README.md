# IRCBase (Go Edition)

IRCBase 现在是一个使用 Go 编写的轻量级 IRC 风格聊天服务，实现了原 Java 版本的客户端/服务端协议。项目包含：

- `pkg/packets` —— 协议数据结构定义。
- `pkg/protocol` —— 数据包编解码与注册逻辑。
- `pkg/server` —— 可运行的聊天服务器实现。
- `pkg/client` —— 客户端传输层，提供与服务器交互的 Go API。
- `cmd/server` —— 通过命令行启动服务器的入口程序。

## 功能

- 长连接 TCP 通信，采用长度前缀的 JSON 数据包格式。
- 握手、聊天消息、断线通知以及游戏内昵称同步。
- 线程安全的用户管理与周期性在线列表同步。

## 构建与运行

```bash
# 获取依赖（本项目仅使用标准库）
go mod tidy

# 运行服务器，默认监听 0.0.0.0:8888
go run ./cmd/server
```

服务器会在收到 `SIGINT`/`SIGTERM` 信号时优雅退出。

## 客户端使用示例

```go
package main

import (
    "log"

    "github.com/cubk/ircbase/pkg/client"
)

type demoHandler struct{}

func (demoHandler) OnMessage(sender, message string) {
    log.Printf("%s >> %s", sender, message)
}

func (demoHandler) OnDisconnected(reason string) {
    log.Printf("disconnected: %s", reason)
}

func (demoHandler) OnConnected() {
    log.Println("connected to server")
}

func (demoHandler) InGameUsername() string {
    return "DemoIGN"
}

func main() {
    transport, err := client.NewTransport("127.0.0.1:8888", demoHandler{})
    if err != nil {
        log.Fatal(err)
    }
    defer transport.Close()

    transport.Connect("DemoUser", "token")
    transport.SendChat("Hello IRCBase!")
}
```

客户端会自动维护用户名和 IGN 的映射关系，并且每 5 秒同步一次游戏内昵称。

## 协议兼容性

数据包 ID 与字段名称保持与历史 Java 实现一致，因此现有客户端/服务端可以逐步迁移到 Go 版本。
