# Docs Hygiene

> 状态：已实现 · 已与 `scripts/check-docs-links.py`、`make docs-hygiene` 和 `docs/CONTRIBUTING-DOCS.md` 状态标签约定核对。

---

## 1. 本文回答

本文回答 8 个问题：

- IAM 为什么需要 Docs Hygiene？
- Docs Hygiene 检查什么，不检查什么？
- active docs 与 `_archive/` 的边界是什么？
- 退役路径、旧路由、旧术语如何防回流？
- 文档状态标签如何使用？
- 修改目录或文件名后应该同步什么？
- docs-hygiene 通过是否等于文档完全正确？
- 修改后应该执行哪些 Verify？

本文是架构护栏中的文档卫生文档。写作约定见 [../CONTRIBUTING-DOCS.md](../CONTRIBUTING-DOCS.md)；文档中心入口见 [../README.md](../README.md)。

---

## 2. 30 秒结论

Docs Hygiene 把文档体系当成工程资产治理，而不是手写 README 后的手工巡检。

它当前做两类检查：

```text
1. active Markdown 相对链接是否存在；
2. active docs 是否引用退役路径、旧路由、旧术语。
```

它不替代：

```text
业务解释是否与源码一致（需人工回链代码事实源）；
OpenAPI/proto 字段是否完整（看机器契约）；
宣讲口径是否准确（看 06-宣讲 与事实层对齐）。
```

如果只记一句话：

> docs-hygiene 是最低门槛的文档防漂移，不是完整质量保证。

---

## 3. 检查范围

默认扫描：

```text
README.md
docs/（不含 docs/_archive/）
api/
pkg/sdk/README.md
pkg/sdk/docs/
```

`_archive/` 默认不参与检查，只能作为历史追溯，不能作为当前事实源。

---

## 4. 退役事实拦截

`scripts/check-docs-links.py` 维护 `RETIRED_REFERENCES` 列表，会拦截 active docs 中对已退役包路径、旧 HTTP 路由和旧领域术语的引用。

新增退役路径时，应同步更新脚本中的 `RETIRED_REFERENCES` 常量。

---

## 5. 状态标签

| 标签 | 含义 |
| --- | --- |
| `已实现` | 已有代码、契约或测试证据支撑 |
| `待补证据` | 方向可能正确，但尚未完成事实核对 |
| `规划改造` | 后续演进方向，不能写成当前已实现事实 |

---

## 6. 修改文档后的最小同步清单

```text
1. 更新所在目录 README 链接；
2. 更新跨目录回链；
3. 运行 make docs-hygiene；
4. 必要时更新 Verify 命令和代码事实源表；
5. 核对状态标签是否仍准确。
```

---

## 7. 代码事实源

| 事实 | 路径 |
| --- | --- |
| docs hygiene 脚本 | `scripts/check-docs-links.py` |
| Make target | `Makefile` -> `docs-hygiene` |
| 写作约定 | `docs/CONTRIBUTING-DOCS.md` |
| 文档中心入口 | `docs/README.md` |
| 归档区说明 | `docs/_archive/README.md` |

---

## 8. Verify

```bash
make docs-hygiene
```

---

## 9. 本文总结

Docs Hygiene 保证 active docs 的链接、入口和退役事实引用不漂移。它通过 CI 可执行的脚本，把“文档也是事实源的一部分”变成可验证规则，而不是依赖记忆。
