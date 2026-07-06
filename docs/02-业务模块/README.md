# 02-业务模块

## 本目录定位

`02-业务模块/` 是 IAM 文档体系的核心事实层。它按模块讲当前实现，而不是按历史演进讲旧目录。

模块分为：

- 核心模块：Identity、AuthN、AuthZ。
- 辅助模块：IDP、Suggest。

## 阅读顺序

建议先读 [00-模块协作总图.md](00-模块协作总图.md)，再进入具体模块：

| 模块 | 入口 | 定位 |
| --- | --- | --- |
| Identity | [01-Identity](01-Identity/README.md) | 身份事实中心 |
| AuthN | [02-AuthN](02-AuthN/README.md) | 认证域 |
| AuthZ | [03-AuthZ](03-AuthZ/README.md) | 授权域 |
| IDP | [04-IDP](04-IDP/README.md) | 外部身份源辅助模块 |
| Suggest | [05-Suggest](05-Suggest/README.md) | Profile 联想搜索读模型 |

## 模块写法

每个模块先讲领域模型，再讲关键链路：

```text
模块定位
  -> 领域模型
  -> 领域模型图
  -> 核心对象生命周期
  -> 关键链路
  -> 模块边界
  -> 分层架构与代码索引
```

专题设计和取舍放在 [05-专题设计](../05-专题设计/README.md)。
