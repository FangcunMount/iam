# Docs Hygiene

> 状态：待补证据 · 骨架初稿，待与代码/契约核对。

## 30 秒结论

`docs-hygiene` 检查 active Markdown 链接和退役事实引用，防止旧路径、旧术语和 archive 事实回流。

## 事实源

- `../../scripts/check-docs-links.py`
- `../../Makefile`

## 规则

- `_archive/` 默认跳过检查。
- active docs 不引用旧目录来证明当前事实。
- 改目录后先修链接，再提交。

## Verify

```bash
make docs-hygiene
```
