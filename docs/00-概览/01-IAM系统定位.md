# IAM 系统定位

## 本文回答

IAM 是什么、解决什么问题，以及它不应该被理解成什么。

## 30 秒结论

IAM 是面向业务系统接入的身份与访问管理服务。它不是普通用户中心、单纯登录系统、权限 CRUD、微信登录适配器或 Profile 搜索服务。

IAM 的核心边界是：

| 问题 | 模块 |
| --- | --- |
| 用户是谁 | Identity |
| 如何证明用户身份 | AuthN |
| 用户能访问什么 | AuthZ |
| 外部身份来源如何接入 | IDP |
| 如何快速搜索可见 Profile | Suggest |

## 系统定位

IAM 把身份事实、认证事实、授权事实和接入契约分开治理：

- Identity 维护稳定身份主体和业务档案关系。
- AuthN 证明调用者身份并签发访问凭证。
- AuthZ 判断 Subject 能否对 Resource 执行 Action。
- IDP 管理外部身份源配置和外部身份声明。
- Suggest 基于 Profile 构建联想搜索读模型。

## 事实源

- 运行事实：`../internal/apiserver`
- REST 契约：`../api/rest`
- gRPC 契约：`../api/grpc`
- SDK 公开 API：`../pkg/sdk`
