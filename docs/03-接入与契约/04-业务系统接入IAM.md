# 业务系统接入 IAM

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 典型接入路径

```text
业务系统
  -> 选择 REST / gRPC / Go SDK
  -> 完成认证接入
  -> 接入授权 Check
  -> 按需接入 Identity / IDP / Suggest 能力
```

## 选择建议

| 场景 | 入口 |
| --- | --- |
| Web/App/管理端 HTTP 调用 | REST |
| 可信服务间调用 | gRPC |
| Go 服务端集成 | Go SDK |

## 模块回链

- 认证语义见 [../02-业务模块/02-AuthN](../02-业务模块/02-AuthN/README.md)。
- 授权语义见 [../02-业务模块/03-AuthZ](../02-业务模块/03-AuthZ/README.md)。
- 身份事实见 [../02-业务模块/01-Identity](../02-业务模块/01-Identity/README.md)。
- 第三方身份源见 [../02-业务模块/04-IDP](../02-业务模块/04-IDP/README.md)。
- Profile 联想搜索见 [../02-业务模块/05-Suggest](../02-业务模块/05-Suggest/README.md)。
