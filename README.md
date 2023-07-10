# API文档

本文档提供了使用该负载君恩和能够API的方法和端点的详细说明。

## 身份验证

### 身份验证中间件流程

1. 从请求头中获取`Authorization`字段的值。
2. 检查`Authorization`字段的值是否以`"Bearer"`开头。
   - 如果不是，则返回错误信息："authorization header should start with 'Bearer'"（HTTP状态码403）。
3. 去除`Authorization`字段值开头的`"Bearer"`和前后的空格。
4. 将剩余的值与预先设置的身份验证配置进行比较。
   - 如果不匹配，则返回错误信息："wrong authorization header"（HTTP状态码403）。
5. 如果身份验证通过，则返回`nil`。

## 上游管理

### 获取所有上游

- URL: `/admin/upstreams`
- 方法: GET
- 权限要求: 需要进行身份验证
- 返回数据类型: JSON
- 请求示例: 
  ```bash
  curl -X GET -H "Authorization: Bearer access_token" http://localhost:8080/admin/upstreams
  ```
- 返回示例:
  ```json
  [
    {
      "ID": 1,
      "SK": "sk_value",
      "Endpoint": "endpoint_value"
    },
    {
      "ID": 2,
      "SK": "sk_value",
      "Endpoint": "endpoint_value"
    }
  ]
  ```

### 创建新的上游

- URL: `/admin/upstreams`
- 方法: POST
- 权限要求: 需要进行身份验证
- 请求数据类型: JSON
- 请求示例:
  ```bash
  curl -X POST -H "Authorization: Bearer access_token" -H "Content-Type: application/json" -d '{"SK": "sk_value", "Endpoint": "endpoint_value"}' http://localhost:8080/admin/upstreams
  ```
- 返回数据类型: JSON
- 返回示例:
  ```json
  {
    "message": "success"
  }
  ```

### 删除指定ID的上游

- URL: `/admin/upstreams/:id`
- 方法: DELETE
- 权限要求: 需要进行身份验证
- 返回数据类型: JSON
- 请求示例:
  ```bash
  curl -X DELETE -H "Authorization: Bearer access_token" http://localhost:8080/admin/upstreams/1
  ```
- 返回示例:
  ```json
  {
    "message": "success"
  }
  ```

### 更新指定ID的上游

- URL: `/admin/upstreams/:id`
- 方法: PUT
- 权限要求: 需要进行身份验证
- 请求数据类型: JSON
- 请求示例:
  ```bash
  curl -X PUT -H "Authorization: Bearer access_token" -H "Content-Type: application/json" -d '{"SK": "sk_value", "Endpoint": "endpoint_value"}' http://localhost:8080/admin/upstreams/1
  ```
- 返回数据类型: JSON
- 返回示例:
  ```json
  {
    "message": "success"
  }
  ```