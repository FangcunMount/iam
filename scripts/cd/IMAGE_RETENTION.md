# 发布镜像保留

发布脚本在目标服务器取得共享锁，然后执行镜像导入/拉取、容器更新和现有验证。
验证成功后调用 `image-retention.py --apply`；清理失败输出 warning，不将健康发布回滚。
工作流会先执行 `python3 scripts/cd/test_image_retention.py`。

## 保留规则

- 每个服务保留当前成功版本及最近两个不同的成功镜像 ID；重复部署和回滚按成功执行顺序更新。
- 保护所有现有容器（包含停止的容器）的镜像引用。
- 首次接入保护现存最新三个镜像及替换前容器使用的镜像；这些只记录为 bootstrap，不能当作成功发布证据。
  积累三个成功版本后取消 bootstrap 保护。因此过渡期/容器引用可能使保留数量超过三个。
- `fallback-`、`m6-rollback-` 标签是运维明确保留的回退工件，不因三个成功版本轮换而自动删除；退役须另行审查。
- 同一镜像的 ACR/GHCR 标签合并计算；只删除当前服务允许仓库中的旧标签。
  带未知仓库别名的镜像、无标签镜像、工具镜像和数据卷均不参与本脚本的删除。
- 使用 `docker image rm --no-prune`，不强制删除，不运行全局 system/image prune。

## 锁与记录

所有部署入口共用 `/var/lib/fangcun-image-retention/deploy.lock`，覆盖导入到清理全程。
目录 root 所有，0755；锁文件 0666 允许不同部署账号锁定同一个 inode，不能删除或替换锁文件。
版本记录和最近清理记录 root 所有，0600：

- `<service>.json`：成功版本与过渡期保护列表，原子更新。
- `<service>-last-cleanup.json`：候选、已删标签、结果和磁盘可用字节数。

独立调用默认只预览，不更新记录也不删除；省略 `--deployment-locked`，让脚本自己取得同一把锁。
该内部参数仅供已经持锁的发布入口使用。独立调用的 image-ref 必须是已完成验证且正在运行的版本。
IAM 在 serverB 的发布账号不能以 root 执行临时包中的 Python。运维须把已审核的脚本安装在
`/usr/local/libexec/fangcun/image-retention.py`，确保文件与父目录均由 root 持有、发布账号不可改写，
再给发布账号配置仅执行该固定路径的 sudo 规则。发布脚本在调用前逐字节比较发布包与固定脚本；
缺失或不一致只告警，不清理镜像。**不得**为 `/tmp/*/image-retention.py` 配置通配 sudo 规则。
升级清理策略时先由运维更新固定脚本、核对摘要和 sudo 规则，再部署引用它的新发布包。

独立只读预览必须使用这个受信固定路径，例如在首次新版发布初始化目录后：

```bash
sudo python3 /usr/local/libexec/fangcun/image-retention.py --service iam --image-ref REPOSITORY:TAG
```

需要应用预览时增加 `--apply`。成功记录写入失败时不删除；删除遇到引用冲突或容器状态变化时停止并记录失败。
脚本依赖服务器 Python 3、Docker CLI，Shell 发布还依赖 flock。缺少部署锁前置条件会阻止开始发布。

## 交付与维护

标准库脚本随四个仓库的发布包交付，避免生产发布时动态下载可变脚本。IAM 当前需额外校验上述固定安装副本；其他仓库的固定入口接入仍待分别实施。
修改策略时同步 qs-server、iam、qs-operating-system、qs-ai 中的脚本和测试。
本改动不安装定时任务，不调整远程镜像仓库策略，不清理发布目录或备份。
合并并成功执行新版 CI/CD 后才在生产生效；本地测试通过不代表生产已接入。
