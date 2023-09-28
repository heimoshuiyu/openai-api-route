# API 文档

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
  -add
        添加一个 OpenAI 上游
  -addr string
        监听地址（默认为 ":8888"）
  -database string
        数据库地址（默认为 "./db.sqlite"）
  -endpoint string
        OpenAI API 基地址（默认为 "https://api.openai.com/v1"）
  -list
        列出所有上游
  -noauth
        不检查传入的授权头
  -sk string
        OpenAI API 密钥（sk-xxxxx）
```

您可以直接运行 `./openai-api-route` 命令，如果数据库不存在，系统会自动创建。

### 上游管理

您可以使用以下命令添加一个上游：

```bash
./openai-api-route -add -sk sk-xxxxx -endpoint https://api.openai.com/v1
```

您也可以使用 `/admin/upstreams` 的 HTTP 接口进行控制。

另外，您还可以直接编辑数据库中的 `openai_upstreams` 表。

## 身份验证

### 身份验证中间件流程

1. 从请求头中获取`Authorization`字段的值。
2. 检查`Authorization`字段的值是否以`"Bearer"`开头。
   - 如果不是，则返回错误信息："authorization header should start with 'Bearer'"（HTTP 状态码 403）。
3. 去除`Authorization`字段值开头的`"Bearer"`和前后的空格。
4. 将剩余的值与预先设置的身份验证配置进行比较。
   - 如果不匹配，则返回错误信息："wrong authorization header"（HTTP 状态码 403）。
5. 如果身份验证通过，则返回`nil`。

## 上游管理

没什么好说的，直接操作数据库 `openai_upstreams` 表，改动立即生效
