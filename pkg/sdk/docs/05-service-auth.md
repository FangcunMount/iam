# 服务间认证：mTLS、方法准入与宿主生命周期

> 状态：已实现 · Go module为v5；当前AuthN v3、AuthZ v4、Identity/IDP v2是各自线协议版本。

服务间身份来自mTLS客户端证书，方法ACL决定可调用的RPC；服务证书不代替用户凭证、被委托subject来源、对象关系或数据范围。直接调用SDK无需申请服务JWT，当前没有ServiceToken签发/续期helper。

以下是装配片段，ctx与证书路径来自宿主；完整单独编译的接入骨架见[宿主接入正文](../../../docs/04-接口与SDK/02-Go-SDK与业务系统接入.md)。

```go
client, err := sdk.NewClient(ctx, &sdk.Config{
    Endpoint: "iam.example.com:9090",
    TLS: &sdk.TLSConfig{
        Enabled: true,
        ServerName: "iam.example.com",
        CACert: "/etc/iam/ca.pem",
        ClientCert: "/etc/iam/client.pem",
        ClientKey: "/etc/iam/client-key.pem",
    },
    Retry: &sdk.RetryConfig{Enabled: false},
})
if err != nil { return err }
// 宿主保存并复用client，在停止新请求、处理在途调用后Close。
```

构造默认非阻塞，不证明握手或方法准入。每次实际RPC设置deadline并检查结果；一个读方法成功不证明写方法或全部服务已接受。Retry=false只不生成SDK默认策略，resolver/其他选项与透明重试另按[重试合同](../../../docs/04-接口与SDK/02-Go-SDK与业务系统接入.md#4-一个可编译的服务端接入骨架)核对。

SDK在构造TLS时加载客户端证书，没有自动重读文件任务。宿主换证需重新构造受控Client、验证新连接上的所需方法，再按宿主预算停止旧调用和关闭旧连接；原有连接与IAM服务器证书重载是不同生命周期。JWKS Manager、独立HTTP及观测资源不由Client.Close关闭，借用资源不能随意关闭。CA/身份解析/ACL/旧连接边界见[传输安全](../../../docs/03-基础设施/05-传输层与服务间安全.md)。

## 退役能力与升级边界

ServiceToken、IssueServiceToken、ServiceAuthHelper已退役；旧服务Token不能当用户AccessToken，当前消费者不得继续调用已删除RPC。按实际使用能力核对module/API版本与服务端注册，显式升级消费者及证书准入。需要协调的是退役能力的依赖集合，不把各模块协议版本泛化为同一天统一变化；整体发布/回滚政策另由[退役发布手册](../../../docs/05-工程质量与运维/07-IAM-v4服务认证退役与统一发布.md)维护。此页不提供生产操作授权或接受证明。
