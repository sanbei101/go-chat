---
title: 默认模块
language_tabs:
  - shell: Shell
  - http: HTTP
  - javascript: JavaScript
  - ruby: Ruby
  - python: Python
  - php: PHP
  - java: Java
  - go: Go
toc_footers: []
includes: []
search: true
code_clipboard: true
highlight_theme: darkula
headingLevel: 2
generator: "@tarslib/widdershins v4.0.30"

---

# 默认模块

Base URLs:

* <a href="http://localhost:8800">开发环境: http://localhost:8800</a>

# Authentication

- HTTP Authentication, scheme: bearer

# Default

## POST 注册

POST /signup

> Body 请求参数

```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "123456"
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|body|body|object| 是 |none|
|» username|body|string| 是 |none|
|» email|body|string| 是 |none|
|» password|body|string| 是 |none|

> 返回示例

> 200 Response

```json
{
  "id": "1",
  "username": "alice",
  "email": "alice@example.com"
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

状态码 **200**

|名称|类型|必选|约束|中文名|说明|
|---|---|---|---|---|---|
|» id|string|true|none||none|
|» username|string|true|none||none|
|» email|string|true|none||none|

## POST 登入

POST /login

> Body 请求参数

```json
{
  "email": "alice@example.com",
  "password": "123456"
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|body|body|object| 是 |none|
|» email|body|string| 是 |none|
|» password|body|string| 是 |none|

> 返回示例

> 200 Response

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjEiLCJ1c2VybmFtZSI6ImFsaWNlIiwiaXNzIjoiMSIsImV4cCI6MTc3NDMxNDAwMH0.fPAcJxON6AHEoW2lE2eQQiujHX3icL0J5VMN7CUphZg",
  "username": "alice",
  "id": "1"
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

状态码 **200**

|名称|类型|必选|约束|中文名|说明|
|---|---|---|---|---|---|
|» accessToken|string|true|none||none|
|» username|string|true|none||none|
|» id|string|true|none||none|

## GET 校验用户

GET /ws/auth

> 返回示例

> 200 Response

```json
{}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

## POST 创建聊天室

POST /ws/createRoom

> Body 请求参数

```json
{
  "name": "general"
}
```

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|body|body|object| 是 |none|
|» name|body|string| 是 |none|

> 返回示例

> 200 Response

```json
{
  "id": "1",
  "name": "general"
}
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

状态码 **200**

|名称|类型|必选|约束|中文名|说明|
|---|---|---|---|---|---|
|» id|string|true|none||none|
|» name|string|true|none||none|

## GET 获取房间在线用户列表

GET /ws/getClients/{roomId}

### 请求参数

|名称|位置|类型|必选|说明|
|---|---|---|---|---|
|roomId|path|integer| 是 |none|

> 返回示例

> 200 Response

```json
[
  {
    "id": "1",
    "username": "alice"
  }
]
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

状态码 **200**

|名称|类型|必选|约束|中文名|说明|
|---|---|---|---|---|---|
|» id|string|false|none||none|
|» username|string|false|none||none|

## GET 获取聊天室列表

GET /ws/getRooms

> 返回示例

> 200 Response

```json
[
  {
    "id": "1",
    "name": "general"
  }
]
```

### 返回结果

|状态码|状态码含义|说明|数据模型|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|

### 返回数据结构

状态码 **200**

|名称|类型|必选|约束|中文名|说明|
|---|---|---|---|---|---|
|» id|string|false|none||none|
|» name|string|false|none||none|

# 数据模型

