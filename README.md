# openai-api-route 文档

这是一个 OpenAI API 负载均衡的简易工具，使用 golang 原生 reverse proxy 方法转发请求到 OpenAI 上游。遇到上游返回报错或请求超时会自动按顺序选择下一个上游进行重试，直到所有上游都请求失败。

功能包括：

- 自定义 Authorization 验证头
- 支持所有类型的接口 (`/v1/*`)
- 提供 Prometheus Metrics 统计接口 (`/v1/metrics`)
- 按照定义顺序请求 OpenAI 上游
- 识别 ChatCompletions Stream 请求，针对 Stream 请求使用 5 秒超时。对于其他请求使用 60 秒超时。
- 记录完整的请求内容、使用的上游、IP 地址、响应时间以及 GPT 回复文本
- 请求出错时发送 飞书 或 Matrix 消息通知

本文档详细介绍了如何使用负载均衡和能力 API 的方法和端点。

## 部署方法

### 编译

以下是编译和运行该负载均衡 API 的步骤：

1. 首先，确保您已经安装了 golang 和 gcc。

2. 克隆本仓库到您的本地机器上。

3. 打开终端，并进入到仓库目录中。

4. 在终端中执行以下命令来编译代码：

   ```
   make
   ```

   这将会编译代码并生成可执行文件。

5. 编译成功后，您可以直接运行以下命令来启动负载均衡 API：

   ```
   ./openai-api-route
   ```

   默认情况下，API 将会在本地的 8888 端口进行监听。

   如果您希望使用不同的监听地址，可以使用 `-addr` 参数来指定，例如：

   ```
   ./openai-api-route -addr 0.0.0.0:8080
   ```

   这将会将监听地址设置为 0.0.0.0:8080。

6. 如果数据库不存在，系统会自动创建一个名为 `db.sqlite` 的数据库文件。

   如果您希望使用不同的数据库地址，可以使用 `-database` 参数来指定，例如：

   ```
   ./openai-api-route -database /path/to/database.db
   ```

   这将会将数据库地址设置为 `/path/to/database.db`。

7. 现在，您已经成功编译并运行了负载均衡和能力 API。您可以根据需要添加上游、管理上游，并使用 API 进行相关操作。

### 运行

以下是运行命令的用法：

```
Usage of ./openai-api-route:
  -addr string
        监听地址（默认为 ":8888"）
  -upstreams string
        上游配置文件（默认为 "./upstreams.yaml"）
  -dbtype
        数据库类型 (sqlite 或 postgres，默认为 sqlite)
  -database string
        数据库地址（默认为 "./db.sqlite"）
        如果数据库为 postgres ，则此值应 PostgreSQL DSN 格式
        例如 "host=127.0.0.1 port=5432 user=postgres dbname=openai_api_route sslmode=disable password=woshimima"
  -list
        列出所有上游
  -noauth
        不检查传入的授权头
```

以下是一个 `./upstreams.yaml` 文件配置示例

```yaml
authorization: woshimima
upstreams:
  - sk: "secret_key_1"
    endpoint: "https://api.openai.com/v2"
  - sk: "secret_key_2"
    endpoint: "https://api.openai.com/v1"
    timeout: 30
```

请注意，程序会根据情况修改 timeout 的值

您可以直接运行 `./openai-api-route` 命令，如果数据库不存在，系统会自动创建。
