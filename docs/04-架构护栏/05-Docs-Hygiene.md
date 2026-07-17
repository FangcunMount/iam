# Docs Hygiene

> 状态：已实现 · 已与 `scripts/check-docs-links.py`、`Makefile` 和 CI 核对。

## 1. 结论

`make docs-hygiene` 是 IAM 文档的最低防漂移门槛。它检查链接、可验证的仓库路径和退役事实，并在 CI 中执行。

它不能证明业务解释完全正确；代码语义、契约字段和运行行为仍需人工回链事实源。

## 2. 检查范围

默认扫描：

```text
README.md
docs/ 下的 active Markdown
docs/_archive/README.md
api/ 下的 Markdown
pkg/sdk/README.md
pkg/sdk/docs/
```

归档索引参与检查，确保清单中的材料真实存在；`_archive/` 内的历史正文被排除，因为它不应持续追随当前代码。

## 3. 当前检查项

| 检查 | 目的 |
| --- | --- |
| Markdown 本地链接 | 防止入口和跨文档导航失效 |
| 反引号中的可验证仓库路径 | 防止代码文件、相对目录和 Verify 路径失效 |
| `RETIRED_REFERENCES` | 防止已删除包、旧路由和退役术语回流 |

代码路径检查只处理能够可靠判断的相对路径和带源码后缀的仓库路径。迁移说明中故意提到的旧目录不会仅因不存在而失败；明确退役且禁止回流的事实应加入 `RETIRED_REFERENCES`。

## 4. 不在自动检查范围内

```text
业务解释是否与源码一致；
标题锚点是否保持兼容；
OpenAPI/proto 字段是否与运行时完全一致；
规划方案是否已经真正实现；
宣讲材料是否完整覆盖事实层。
```

这些内容依靠代码评审、契约验证、模块测试和状态标签共同约束。

## 5. 维护规则

- 新增明确退役的路径、路由或术语时，更新脚本中的 `RETIRED_REFERENCES`。
- 调整扫描范围或例外时，同步更新本文和 `docs/CONTRIBUTING-DOCS.md`。
- 不要通过扩大忽略范围来掩盖 active docs 的真实失效链接。
- CI 失败时先修正文档事实；只有确认是误报时才收窄规则。

## 6. Verify

```bash
make docs-hygiene
```

事实源：

| 事实 | 路径 |
| --- | --- |
| 校验脚本 | `scripts/check-docs-links.py` |
| Make target | `Makefile` |
| CI | `.github/workflows/ci.yml` |
| 写作约定 | `docs/CONTRIBUTING-DOCS.md` |
| 归档规则 | `docs/_archive/README.md` |
