// Package organization 定义业务组织 ID 的跨模块约定（非 IAM 身份域）。
// IAM 仅通过 JWT org_id claim 透传；默认组织 ID 供读模型占位、测试与业务侧对齐。
package organization

// DefaultID 业务默认组织 ID（与 QS operating 默认 org 对齐）。
const DefaultID uint64 = 1
