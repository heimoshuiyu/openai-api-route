# openai-api-route 文档

这是一个 OpenAI API 负载均衡的简易工具，使用 golang 原生 reverse proxy 方法转发请求到 OpenAI 上游。遇到上游返回报错或请求超时会自动按顺序选择下一个上游进行重试，直到所有上游都请求失败。

功能包括：

- 自定义 Authorization 验证头
- 支持所有类型的接口 (`/v1/*`)
- 提供 Prometheus Metrics 统计接口 (`/v1/metrics`)
- 按照定义顺序请求 OpenAI 上游，出错或超时自动按顺序尝试下一个
- 识别 ChatCompletions Stream 请求，针对 Stream 请求使用更短的超时。具体超时策略请参阅 [超时策略](#超时策略) 一节
- 有选择地记录请求内容、请求头、使用的上游、IP 地址、响应时间以及响应等内容。具体记录策略请参阅 [记录策略](#记录策略) 一节
- 请求出错时发送 飞书 或 Matrix 平台的消息通知
- 支持 Replicate 平台上的 mistral 模型（beta）

本文档详细介绍了如何使用负载均衡和能力 API 的方法和端点。

## 配置文件

默认情况下程序会使用当前目录下的 `config.yaml` 文件，您可以通过使用 `-config your-config.yaml` 参数指定配置文件路径。

以下是一个配置文件示例，你可以在 `config.sample.yaml` 文件中找到同样的内容

```yaml
authorization: woshimima

# 默认超时时间，默认 120 秒，流式请求是 10 秒
timeout: 120
stream_timeout: 10 

# 使用 sqlite 作为数据库储存请求记录
dbtype: sqlite
dbaddr: ./db.sqlite

# 使用 postgres 作为数据库储存请求记录
# dbtype: postgres
# dbaddr: "host=127.0.0.1 port=5432 user=postgres dbname=openai_api_route sslmode=disable password=woshimima"

upstreams:
  - sk: hahaha
    endpoint: "https://localhost:8888/v1"
    allow:
      # whisper 等非 JSON API 识别不到 model，则使用 URL 路径作为模型名称
      - /v1/audio/transcriptions

  - sk: "secret_key_1"
    endpoint: "https://api.openai.com/v2"
    timeout: 120  # 请求超时时间，默认120秒
    stream_timeout: 10  # 如果识别到 stream: true, 则使用该超时时间
    allow:  # 可选的模型白名单
      - gpt-3.5-trubo
      - gpt-3.5-trubo-0613

  # 您可以设置很多个上游，程序将依次按顺序尝试
  - sk: "secret_key_2"
    endpoint: "https://api.openai.com/v1"
    timeout: 30
    deny: 
      - gpt-4

  - sk: "key_for_replicate"
    type: replicate
    allow:
      - mistralai/mixtral-8x7b-instruct-v0.1
```

### 配置多个验证头

您可以使用英文逗号 `,` 分割多个验证头。每个验证头都是有效的，程序会记录每个请求使用的验证头

```yaml
authorization: woshimima,iampassword
```

您也可以为上游单独设置验证头

```yaml
authorization: woshimima,iampassword
upstreams:
  - sk: key
    authorization: woshimima
```

如此，只有携带 `woshimima` 验证头的用户可以使用该上游。

### 复杂配置示例

```yaml

# 默认验证头
authorization: woshimima

upstreams:

  # 允许所有人使用的文字转语音
  - sk: xxx
    endpoint: http://localhost:5000/v1
    noauth: true
    allow:
      - /v1/audio/transcriptions
    
  # guest 专用的 gpt-3.5-turbo-0125 模型
  - sk: 
    endpoint: https://api.xxx.local/v1
    authorization: guest
    allow:
      - gpt-3.5-turbo-0125
```

## 部署方法

有两种推荐的部署方法：

1. 使用预先构建好的容器 `docker.io/heimoshuiyu/openai-api-route:latest`
2. 自行编译

### 使用容器运行

> 注意，如果您使用 sqlite 数据库，您可能还需要修改配置文件以将 SQLite 数据库文件放置在数据卷中。

```bash
docker run -d --name openai-api-route -v /path/to/config.yaml:/config.yaml docker.io/heimoshuiyu/openai-api-route:latest
```

使用 Docker Compose

```yaml
version: '3'
services:
  openai-api-route:
    image: docker.io/heimoshuiyu/openai-api-route:latest
    ports:
      - 8888:8888
    volumes:
      - ./config.yaml:/config.yaml
```

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

## 模型允许与屏蔽列表

如果对某个上游设置了 allow 或 deny 列表，则负载均衡只允许或禁用用户使用这些模型。负载均衡程序会先判断白名单，再判断黑名单。

如果你混合使用 OpenAI 和 Replicate 平台的模型，你可能需要分别为 OpenAI 和 Replicate 上游设置他们各自的允许列表，否则用户请求 OpenAI 的模型时可能会发送到 Replicate 平台

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