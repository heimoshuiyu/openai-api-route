# openai-api-route 文档

这是一个 OpenAI API 负载均衡的简易工具，使用 golang 原生 reverse proxy 方法转发请求到 OpenAI 上游。遇到上游返回报错或请求超时会自动按顺序选择下一个上游进行重试，直到所有上游都请求失败。

功能包括：

- 自定义 Authorization 验证头
- 支持所有类型的接口 (`/v1/*`)
- 提供 Prometheus Metrics 统计接口 (`/v1/metrics`)
- 按照定义顺序请求 OpenAI 上游
- 识别 ChatCompletions Stream 请求，针对 Stream 请求使用 5 秒超时。具体超时策略请参阅 [超时策略](#超时策略) 一节
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

# 使用 sqlite 作为数据库储存请求记录
dbtype: sqlite
dbaddr: ./db.sqlite

# 使用 postgres 作为数据库储存请求记录
# dbtype: postgres
# dbaddr: "host=127.0.0.1 port=5432 user=postgres dbname=openai_api_route sslmode=disable password=woshimima"

upstreams:
  - sk: "secret_key_1"
    endpoint: "https://api.openai.com/v2"
  - sk: "secret_key_2"
    endpoint: "https://api.openai.com/v1"
    timeout: 30
```

请注意，程序会根据情况修改 timeout 的值

您可以直接运行 `./openai-api-route` 命令，如果数据库不存在，系统会自动创建。

## 超时策略

在处理上游请求时，超时策略是确保服务稳定性和响应性的关键因素。本服务通过配置文件中的 `Upstreams` 部分来定义多个上游服务器。每个上游服务器都有自己的 `Endpoint` 和 `SK`（可能是密钥或特殊标识）。服务会按照配置文件中的顺序依次尝试每个上游服务器，直到请求成功或所有上游服务器都已尝试。

### 单一上游配置

当配置文件中只定义了一个上游服务器时，该上游的超时时间将被设置为 120 秒。这意味着，如果请求没有在 120 秒内得到上游服务器的响应，服务将会中止该请求并可能返回错误。

### 多上游配置

如果配置文件中定义了多个上游服务器，服务将会按照定义的顺序依次尝试每个上游。对于每个上游服务器，服务会检查其 `Endpoint` 和 `SK` 是否有效。如果任一字段为空，服务将返回 500 错误，并记录无效的上游信息。

### 超时策略细节

服务在处理请求时会根据不同的条件设置不同的超时时间。超时时间是指服务等待上游服务器响应的最大时间。以下是超时时间的设置规则：

1. **默认超时时间**：如果没有特殊条件，服务将使用默认的超时时间，即 60 秒。

2. **流式请求**：如果请求体被识别为流式（`requestBody.Stream` 为 `true`），并且请求体检查（`requestBodyOK`）没有发现问题，超时时间将被设置为 5 秒。这适用于那些预期会快速响应的流式请求。

3. **大请求体**：如果请求体的大小超过 128KB（即 `len(inBody) > 1024*128`），超时时间将被设置为 20 秒。这考虑到了处理大型数据可能需要更长的时间。

4. **上游超时配置**：如果上游服务器在配置中指定了超时时间（`upstream.Timeout` 大于 0），服务将使用该值作为超时时间。这个值是以秒为单位的。
