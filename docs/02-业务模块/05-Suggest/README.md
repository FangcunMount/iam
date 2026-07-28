# Suggest：Profile 联想搜索读模型

> 状态：已实现 · 派生模型、Full/Delta 刷新、查询安全与当前限制已按代码和测试复核。

Suggest 从 Identity 的 MySQL 事实构建进程内索引，按当前操作员可见范围过滤，再返回脱敏的 Profile 联想结果。它不拥有 User/Profile/ProfileLink，也不替代详情接口的权威授权。

## 阅读路径

1. [模块总览](00-模块总览.md)：建立权威事实、派生索引、可见 scope、Full/Delta 与安全策略的整体模型。
2. [模型与应用端口](01-模型与应用端口.md)：主从关系、查询模型和应用边界。
3. [Full/Delta 索引刷新](02-关键链路-索引刷新Full-Delta.md)：原子切换、tombstone、并发锁和新鲜度。
4. [SuggestProfile 查询](03-关键链路-SuggestProfile查询.md)：授权、可见性、排序、限流和脱敏顺序。
5. [模块边界与代码索引](04-模块边界与代码索引.md)：定位搜索字段、scope、Delta SQL 和集中式索引的完整修改面。
6. [为什么采用派生读模型](../../06-专题设计/05-Suggest为什么是读模型.md)：方案演化和替代设计。

## 主链路

```text
MySQL Identity facts
  -> Full/Delta Loader
  -> atomic runtime / Trie + Hash
  -> REST query
  -> AuthZ + visibility-derived scope
  -> filter -> rank -> limit -> mask
```

## 当前实现要特别记住的四点

- Full 构建新 Store 后原子换指针；Delta 依赖 tombstone 删除旧 keys。
- 可见性过滤发生在排序截断前，索引命中不等于有权读取。
- 默认 Loader 的 OrgID 是过渡占位，不能当成成熟的多组织授权模型。
- 模块只提供 REST，没有 gRPC 或 Go SDK；进程重启后必须 Full 重建。

## 代码入口

- domain/application：`internal/apiserver/domain/suggest`、`internal/apiserver/application/suggest`
- infra：`internal/apiserver/infra/suggest`、`internal/apiserver/infra/mysql/suggest`
- transport/container：`internal/apiserver/transport/rest/suggest`、`internal/apiserver/container/suggest`
- contract：`api/rest/suggest.v2.yaml`

## 验证

```bash
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/apiserver/domain/suggest/... ./internal/apiserver/application/suggest/... ./internal/apiserver/infra/suggest/... ./internal/apiserver/infra/mysql/suggest/... ./internal/apiserver/transport/rest/suggest/... ./internal/apiserver/container/suggest
```
