# boat 运维管理系统

## 项目简介

boat是一个基于 Go 语言开发的企业级运维管理系统，提供主机管理、SSH 堡垒机、Web Terminal、会话录像与审计等功能。系统采用前后端分离架构，支持多种认证方式和细粒度的权限控制。

### 核心特性

- 🖥️ **Web Terminal** - 浏览器直接连接 SSH，支持 xterm.js 全功能终端
- 🔐 **SSH 堡垒机** - 独立的 SSH 服务（端口 2222），支持命令行交互式操作
- 🛡️ **高危指令拦截** - 实时检测并阻止危险命令执行，支持精确/前缀/正则三种匹配模式，内置 14 条防护规则，管理员豁免
- 📹 **会话录像** - 自动录制所有 SSH 会话，支持在线回放和下载
- 👥 **权限管理** - 基于 Casbin 的 RBAC 权限控制，支持细粒度资源授权
- 🔑 **凭证管理** - 支持密码和密钥两种认证方式，凭证加密存储
- 📊 **审计日志** - 完整的操作日志记录，支持日志查询和导出
- 🤖 **OSP Agent 执行机管控** - 自研 OSP 加密协议（自定义端口 9090）：执行机 `osp-agent` 主动反连控制台长连接常驻，控制台下发命令/脚本任务并回收标准输出与退出码，实时监控节点存活与 CPU/内存/磁盘/负载/运行时长（详见 §22）
- 🆘 **应急控制通道** - SSH 登录不进去时（用户密码过期、账户被锁定、sshd 配置错误、sudoers 损坏、防火墙误封），只要 `osp-agent` 进程存活，控制台仍可通过加密通道下发修复命令/脚本，实现"无登录管控"


---

## 系统功能

### 1. 用户管理
- 用户增删改查
- 用户状态管理（启用/禁用）
- 用户角色分配
- 用户部门归属
- 密码修改与重置

### 2. 角色管理
- 角色增删改查
- 角色菜单权限配置
- 角色用户关联
- 角色状态管理

### 3. 菜单管理
- 树形菜单结构
- 菜单权限标识配置
- 菜单图标、排序设置
- 按钮级权限控制

### 4. 部门管理
- 树形部门结构
- 部门增删改查
- 部门状态管理

### 5. 主机管理
- 主机增删改查
- 主机规格配置（CPU、内存、磁盘）
- 主机状态管理
- 主机分组管理
- 主机凭证绑定
- 切换 root（sudo 提权）：支持禁止 root 直登的主机——以普通用户登录后自动 `sudo -i` 切换 root，Web Terminal 与任务执行（含定时任务）均生效；密钥/无密码场景依赖远端 NOPASSWD 配置

### 6. 凭证管理
- 凭证增删改查
- 支持密码认证和密钥认证
- 支持 SSH、RDP、VNC 协议
- 凭证加密存储（RSA 加密）
- 凭证授权管理

### 7. 主机分组
- 分组增删改查
- 分组权限授权
- 分组内主机管理

### 8. 授权管理
- 用户主机授权
- 用户凭证授权
- 主机组授权
- 凭证组授权

### 9. Web Terminal
- 浏览器 SSH 连接
- 支持多终端窗口
- 终端自适应大小
- 终端尺寸前后端同步（连接时即以实际窗口尺寸创建 PTY，实时同步窗口调整）
- 支持复制粘贴
- 会话自动录制

### 10. 会话审计
- 会话列表查询
- 会话详情查看
- 会话录像回放（asciinema v2 格式，前端播放器支持暂停/继续/重新播放/倍速）
- 录像文件下载
- 会话统计分析

### 11. 操作日志
- 用户操作记录
- 登录日志
- 操作类型筛选
- 时间范围查询

### 12. 系统监控
- 在线用户统计
- 系统资源监控
- 会话统计

### 13. 高危指令管理
- 拦截规则增删改查
- 三种匹配模式：精确匹配、前缀匹配、正则匹配
- 内置 14 条防护规则（rm -rf /、mkfs、dd 写磁盘、fork 炸弹、shutdown 等）
- 自定义规则管理（内置规则仅允许启停，不可删除）
- 规则状态启停控制
- Web Terminal 和 SSH 堡垒机双通道实时拦截
- 超级管理员（user ID = 1）豁免拦截

### 14. 脚本管理
- 脚本库增删改查
- 支持 Shell、Python 等脚本类型
- 脚本内容在线编辑与版本管理
- 脚本与任务模板关联
- **脚本上传**：支持从本地上传 `.sh/.bash/.py/.ps1/.txt` 等脚本文件，后端按扩展名自动推断语言（`.py`→Python，其余→Shell），内容写入脚本库（单文件上限 1MB）；也可在上传时附带 `name`/`lang`/`remark` 表单字段覆盖默认值

### 15. 任务模板
- **用途**：把高频运维操作（如批量查磁盘 `df -h`、批量重启服务、跑健康检查脚本）固化为可复用配置，存好命令/脚本/凭证/超时后，在「任务执行」或「定时任务」里选模板即可一键带出配置，只需挑主机就能跑——减少重复录入、保证操作一致性。
- 模板增删改查
- 支持三种任务类型：执行命令、执行脚本、分发文件
- 可配置超时时间、执行凭证、脚本语言
- 模板与脚本库联动
- **引用闭环（已打通）**：「任务执行」与「定时任务」表单均可选择模板，选中自动带出类型/命令/脚本；执行记录写入来源 `templateId` 便于追溯；模板配置了凭证时，执行优先用该凭证连接（不可用则回退默认解析）
- 注：`分发文件(file)` 类型为预留能力，文件分发引擎尚未实现，模板中 file 类型暂不可直接执行

### 16. 任务编排
- 可视化多步骤任务流水线编排
- 支持步骤顺序配置、父子步骤关系
- 步骤失败重试策略（重试次数、失败处理）
- 编排与模板联动执行

### 17. 任务执行
- 五种执行方式：快速命令、快速脚本、快速文件、模板执行、编排执行
- 异步并发执行，支持多主机批量操作
- 实时执行状态追踪（待执行/执行中/已完成/部分失败/全部失败/已取消）
- 单台主机执行结果详情查看
- 执行记录分页查询与时间范围筛选
- 支持手动取消正在执行的任务
- **「需要 root 执行」开关**：执行表单可勾选，后端在执行时自动 `sudo` 切换 root 下发命令（复用 §5 主机 sudo 提权机制）；适用于需 root 权限的操作，需目标主机已配置可提权凭证（密码或 NOPASSWD）

### 18. 定时任务（Cron 调度）
- 基于标准 5 段 Cron 表达式（分 时 日 月 周）的定时调度
- 支持定时执行「命令」或「脚本库脚本」（Python 脚本自动以 `python3 -` 内联下发）
- 可视化启停控制（启用/停用），停用后从调度器注销
- 支持「立即执行一次」，便于调试与临时触发
- 运行状态追踪：记录上次运行时间、下次运行时间（自动按 Cron 推算）、最近一次执行结果
- 服务启动时自动加载所有启用中的定时任务，新增/修改/删除实时生效（无需重启）
- 凭证解析沿用「主机直绑 → 用户授权凭证（§8 授权管理）→ 超级管理员回退」策略，确保定时任务也能正确找到目标主机认证方式
- **「需要 root 执行」开关**：定时任务可勾选，每次调度触发时自动 `sudo` 切换 root 执行（与任务执行共用提权通道）

### 18. SFTP 文件管理
- 远程目录浏览与文件列表查看
- 文件上传（支持大文件流式传输）
- 文件下载
- 远程文件/目录删除
- 远程文件/目录重命名
- 远程目录创建
- 严格的权限校验（非管理员需授权凭证）

### 19. 活跃会话管理
- 实时查看当前所有活跃的 SSH 堡垒机会话
- 管理员可强制终止指定会话
- 会话信息包括用户、目标主机、连接时长等

### 20. 验证码
- 登录图形验证码
- 防止暴力破解与自动化攻击

### 21. IP 白名单（访问控制）
- 网络层入口限制，开启后**仅允许白名单内的 IP / CIDR 网段**访问系统（同时覆盖前端页面静态资源与全部后端 API）
- 配置文件 `security.ip-whitelist-enabled` 开关（默认关闭），`security.ip-whitelist` 为 IP 或网段列表，例如 `192.168.1.0/24`、`10.0.0.1`
- 白名单为前置拦截中间件，在鉴权之前生效；命中失败时统一返回 `403 IP 不在白名单，拒绝访问`
- 启用时务必将自身管理 IP 纳入白名单，否则会被一并拒绝（fail-closed）

### 22. OSP Agent 执行机管控

为执行机（被管节点）提供一条**独立于 SSH 的加密反向管控通道**。控制台监听自定义端口（默认 `9090`）上的自研 **OSP（Ops Service Protocol）** 协议；执行机上的 `osp-agent` 以系统服务常驻，主动外连控制台建立加密长连接，待命接收下发的命令/脚本任务并执行、回传结果，同时周期性上报心跳与系统指标。

与「任务执行 / 定时任务」（基于 SSH 直连目标主机）的本质区别：**OSP 通道由执行机主动外连，控制台无需掌握执行机账号密码**，因此即使执行机 SSH 已不可用，只要 agent 进程存活，控制台依旧能下发指令。

#### 协议与安全
- **传输层**：自定义 TCP 端口（与 HTTP 8080、SSH 堡垒机 2222 完全隔离），非标准协议，抗扫描与未授权接入。
- **身份认证**：接入令牌（token）+ 节点注册信息；令牌由控制台生成、可一键重置、可强制踢下线。
- **密钥协商**：握手阶段 ECDH P-256 协商会话密钥，HKDF-SHA256（salt=`nonceA||nonceB`、info=`boat-osp-v1|token`）派生 32 字节密钥。
- **加密传输**：会话内所有帧经 **AES-256-GCM** 加密，帧头 `seq` 参与 AAD 校验且严格递增，防重放、防篡改。
- **防中间人**：服务端用 RSA-PSS 对 `nonceA||nonceB||ecdhPubB` 签名，agent 端可配置服务端公钥（`server-pubkey`）校验签名，未配置时告警提示（内网测试可用 `-insecure` 跳过）。
- **帧格式**：握手前明文帧 `[4B 长度][1B flag=0][JSON]`；握手后加密帧 `[4B 长度][1B flag=1][8B seq][12B nonce][密文+16B GCM tag]`；单帧上限 8MB。

#### 控制台能力
- **节点管理**：列表 / 详情 / 新增 / 编辑 / 删除；一键重置接入令牌、强制踢下线（断开当前连接）。
- **实时状态监控**：通过 WebSocket 实时推送节点上下线、心跳指标（CPU / 内存 / 磁盘使用率、系统负载、运行时长），前端「执行机节点」页以卡片 + 状态点 + 进度条呈现；超过 `offline-after`（默认 35s）未心跳自动判定离线。
- **任务下发**：向单个 / 多个节点下发「执行命令」或「执行脚本」任务（支持 Shell / Python / PowerShell 三种语言，脚本内容内联或引用脚本库）；可指定超时（默认 60s，上限 600s）、运行超时自动终止；支持手动取消在途任务。
- **结果回收**：每个任务的 stdout / stderr / 退出码 / 执行时长 / 状态（pending/running/success/failed/timeout/cancelled）回传控制台，可在「任务」页查看详情；审计日志留痕。
- **应急工具箱**：内置场景化命令模板（见下方典型场景），快速生成下发内容。

#### 接入与部署
1. **控制台创建节点**：在「执行机管控 → 节点」点击新增，填写名称/标签，保存后获得 `token` 与控制台地址 `SERVER:9090`。
2. **下载 agent 二进制**：控制台提供 Linux / Windows / macOS（amd64 / arm64）构建产物下载（`backend/cmd/agent`，也可 `go build ./cmd/agent` 自行交叉编译）。
3. **获取服务端公钥 `server.pem`**（用于校验握手签名、防中间人）—— 务必取自**正在运行**的服务端，否则密钥对不上会导致 agent 启动报 `服务端签名校验失败（疑似中间人攻击）`：
   - **（推荐）明文接口一键取**：`curl -H "Authorization: Bearer <控制台JWT>" http://<控制台>:8080/api/agent/pubkey -o /opt/osp-agent/server.pem`（返回的就是服务端当前 RSA 公钥的 PEM，与握手签名所用私钥严格成对）。
   - **控制台 API**：`GET /api/agent/nodes/:id/install`，返回 JSON 的 `publicKey` 字段即 PEM 内容，保存到执行机 `/opt/osp-agent/server.pem`。
   - **一键安装脚本**：同一接口的 `script` 字段已自带写入 `server.pem` 并自动注入 `agent.yaml` 的 `server-pubkey`（见下方「方式一」即用此脚本）。
   - **服务端手动导出（仅限非容器部署，且须在运行服务端所在环境）**：`openssl rsa -in configs/rsa_key -pubout -out /opt/osp-agent/server.pem`。⚠️ 若服务端跑在 Docker 内，`configs/rsa_key` 是**容器内**那把，宿主机 `openssl` 导出的不是同一把——必须用 `docker compose exec backend openssl rsa -in configs/rsa_key -pubout` 在容器内导出，或直接用上面的明文接口。
4. **执行机部署**：
   ```bash
   # 方式一：配置文件
   cat > /opt/osp-agent/agent.yaml <<'EOF'
   server: 10.0.0.1:9090      # 控制台 OSP 地址
   token:  osp_xxxxxxxxxxxx   # 控制台下发的接入令牌
   name:   web-prod-01        # 节点名称（留空用主机名）
   labels: web,prod           # 标签（逗号分隔）
   heartbeat: 10              # 心跳周期（秒），服务端可覆盖
   server-pubkey: /opt/osp-agent/server.pem  # 服务端 RSA 公钥（校验握手签名，防中间人）
   EOF
   /opt/osp-agent/osp-agent -c /opt/osp-agent/agent.yaml

   # 方式二：环境变量（容器/K8s 友好）
   OSP_SERVER=10.0.0.1:9090 OSP_TOKEN=osp_xxxxxxxxxxxx osp-agent
   ```
   建议以 systemd 常驻（断线指数退避 1s→60s 自动重连）。
4. **验证**：控制台「执行机节点」页出现该节点为 `online`，即通道就绪。

#### 典型场景：账号异常导致无法 SSH 登录
执行机登录用户**密码过期**或**账户被锁定**（`passwd -l` / `usermod -L` / PAM 策略）、`sshd` 配置错误、`sudoers` 写坏、防火墙误封 22 端口——这些情况下 SSH 已无法进入，但 `osp-agent` 若以服务账号（如 systemd 的 root 或独立 service 用户）运行，进程仍可存活。此时在控制台下发修复命令即可"无登录"完成处置，例如：

```bash
# 场景 1：用户密码过期，重置密码使其可重新 SSH 登录
chage -M 99999 -m 0 -E -1 alice        # 取消过期策略
echo 'alice:NewPass@2026' | chpasswd  # 重置密码

# 场景 2：账户被锁定，解锁
passwd -u alice
usermod -U alice
# 若 PAM 层锁定（faillock），再清失败计数
faillock --user alice --reset

# 场景 3：sudoers 写坏导致 sudo 不可用，推送正确配置
cat > /etc/sudoers.d/ops <<'EOF'
%ops ALL=(ALL) NOPASSWD: ALL
EOF
chmod 440 /etc/sudoers.d/ops
visudo -c   # 校验语法

# 场景 4：sshd 配置错误 / 防火墙误封，恢复 22 端口可达
sed -i 's/^#*Port .*/Port 22/' /etc/ssh/sshd_config
systemctl restart sshd
iptables -D INPUT -p tcp --dport 22 -j DROP 2>/dev/null; true
```

> 说明：以上命令的「执行力」取决于 `osp-agent` 的运行身份。若 agent 以 root（或具备 sudo NOPASSWD 的 service 用户）运行，可直接执行特权操作；否则需在任务中自带提权步骤。生产环境建议 agent 以受限 service 账号运行，敏感修复任务通过审批流程下发（见 §操作审批流）。

#### 验证（端到端冒烟测试）
- 协议与 agent 行为由 `backend/internal/osp/e2e_test.go` 的 `TestE2ESmoke` 端到端验证：以真实 `osp-agent` 二进制为子进程，连接一个复用同源 osp 握手/帧逻辑的测试控制台，覆盖 **加密握手 + 服务端 RSA-PSS 签名校验（防中间人）+ 加密心跳上报 + 任务下发→真实命令执行→结果回收（stdout/退出码透传）**。
- 运行方式（需先构建 agent 二进制）：
  ```bash
  cd backend
  go build -o dist/osp-agent ./cmd/agent          # Linux/macOS 去掉 .exe
  OSP_AGENT_BIN=dist/osp-agent go test ./internal/osp/ -run TestE2ESmoke -v
  ```
- 协议层单测（`TestProtocol*`）另覆盖 ECDH 密钥协商、HKDF 派生一致性、AES-256-GCM 帧编解码与 seq 防重放。

---

## 未来规划

### 短期规划 (v1.1)
- [x] 批量主机导入（Excel/CSV）
- [x] 主机状态自动检测（心跳/在线状态）
- [x] 文件上传下载（SFTP）
- [x] 录像自动清理策略
- [x] 危险命令拦截
- [x] 脚本库与任务执行
- [x] 任务模板与编排
- [x] 活跃会话管理与强制下线

### 中期规划 (v1.2)
- [ ] RDP/VNC 协议支持
- [x] 多因素认证（MFA）
- [x] 操作审批流程
- [ ] 命令审计规则（正则匹配敏感命令告警）
- [ ] 录像对象存储（OSS/S3）
- [x] 定时任务 / Cron 作业调度
- [x] 主机性能监控（OSP Agent 实时上报 CPU/内存/磁盘/负载，离线自动判定）；**告警引擎（阈值触发 / 通知）为后续增强**

### 长期规划 (v2.0)
- [ ] Kubernetes 集群管理
- [ ] 自动化运维（Ansible 集成）
- [ ] Prometheus / Grafana 监控告警集成
- [ ] 多租户支持
- [ ] 高可用部署（多实例 + 负载均衡）
- [ ] 操作工单系统（申请-审批-执行闭环）
- [ ] 数据库审计与敏感数据脱敏

---

## 技术栈

### 后端
| 技术 | 版本 | 说明 |
|------|------|------|
| Go | 1.24+ | 编程语言 |
| Gin | 1.10+ | Web 框架 |
| GORM | 1.31+ | ORM 框架 |
| MySQL | 5.7+ | 关系型数据库 |
| Redis | 5.0+ | 缓存数据库 |
| Casbin | 2.x | 权限控制 |
| JWT | 3.x | 身份认证 |
| WebSocket | - | 实时通信 |
| gliderlabs/ssh | 0.3+ | SSH 服务器 |
| OSP 加密协议 | 自研 | 执行机管控自定义端口协议（ECDH P-256 + AES-256-GCM） |

### 前端
| 技术 | 版本 | 说明 |
|------|------|------|
| Vue | 3.2+ | 前端框架 |
| TypeScript | 5.0+ | 类型支持 |
| Vite | 4.x | 构建工具 |
| Element Plus | 2.3+ | UI 组件库 |
| Pinia | 2.x | 状态管理 |
| Vue Router | 4.x | 路由管理 |
| xterm.js | 5.x | 终端模拟器 |
| Axios | 1.x | HTTP 客户端 |
| Echarts | 5.x | 图表库 |

---

## 环境要求

### 开发环境
- Go 1.24+
- Node.js 16+
- MySQL 5.7+
- Redis 5.0+

### 生产环境
- Go 1.24+
- MySQL 5.7+ / 8.0+
- Redis 6.0+

### 操作系统支持
- Linux (推荐 CentOS 7+, Ubuntu 18.04+)
- macOS
- Windows (开发测试)

---

## 快速开始

### 1. 启动依赖（MySQL + Redis）
```bash
docker-compose up -d
```
> 默认账号：root / Root@123456，库名 boat。如需自定义，修改 `backend/configs/config.yaml`。

### 2. 启动后端
```bash
cd backend
go mod tidy
go run ./cmd/server            # 或 go build -o boat-server ./cmd/server && ./boat-server
```
- 首次启动自动建表、初始化管理员 `admin / admin123`、内置角色/菜单、14 条高危指令规则。
- 接口地址：http://127.0.0.1:8080 ，统一返回 `{code,msg,data}`。

### 3. 启动前端
```bash
cd web
npm install
npm run dev                    # 开发 http://127.0.0.1:5173 （已代理 /api、/ws 到 8080）
# 或生产构建
npm run build && npm run preview
```
> 登录默认账号：`admin / admin123`。

### 4. 目录结构
```
boat/
├── backend/                 # Go + Gin + GORM 后端
│   ├── cmd/server/          # 入口 main.go
│   ├── cmd/agent/           # osp-agent 执行机代理（独立二进制，可交叉编译部署到各节点）
│   ├── configs/             # 配置文件
│   └── internal/
│       ├── config/  database/  model/  utils/
│       ├── casbin/          # RBAC 权限引擎
│       ├── middleware/      # JWT / 权限 / CORS
│       ├── controller/      # 各业务 Handler（含 Web Terminal WS）
│       ├── ssh/             # SSH 客户端（连接/执行/PTY）
│       ├── osp/             # OSP 加密协议 + 服务端监听器/连接 Hub
│       └── service/         # 种子数据
├── web/                     # Vue3 + TS + Vite 前端
│   └── src/{router,store,api,layout,views}
└── docker-compose.yml       # 全栈编排：MySQL + Redis + 后端(python3) + 前端
└── backend/Dockerfile       # 多阶段构建，运行时含 python3 执行环境
└── web/Dockerfile           # 多阶段构建，nginx 托管前端静态产物
```

## 容器化部署（Docker）

全套服务（MySQL + Redis + 后端 + 前端）可通过 Docker Compose 一键编排：

```bash
# 构建并启动全部服务（后端镜像内置 python3 执行环境）
docker compose up -d --build
```

- **后端镜像** `backend/Dockerfile`：多阶段构建，运行时基于 `python:3.12-slim`（内置 python3，供平台本地调用 python3 执行脚本/插件）；除服务程序 `boat` 外还一并编译 `osp-agent` 执行机代理（产物置于 `/app/dist/osp-agent`，供控制台「接入指引」下载）。暴露 `8080`（API）、`2222`（SSH 堡垒机）与 `9090`（OSP Agent 加密通道）。
- **前端镜像** `web/Dockerfile`：构建阶段基于 `node:24-bookworm-slim`（glibc，规避 Node 22/24 Alpine/musl 镜像上 npm 已知崩溃 `Exit handler never called!`），由 `nginx:1.30-alpine` 托管静态产物；`web/nginx.conf` 将 `/api` 反向代理到后端服务 `backend:8080`，并支持 SPA 路由回退。
- **访问入口**：前端 `http://<宿主机>:8088`，后端 API `http://<宿主机>:8080`。
- **配置注入**：后端 `config.go` 支持 `BOAT_` 前缀环境变量覆盖配置文件中的 DB/Redis 主机——`BOAT_MYSQL_HOST` / `BOAT_MYSQL_PORT` / `BOAT_REDIS_HOST` / `BOAT_REDIS_PORT`；compose 中已默认指向 `mysql` / `redis` 服务名，无需改动 `config.yaml`。
- **默认账号**：平台 `admin / admin123`；MySQL `root / Root@123456`（与 `config.yaml`、redis `requirepass 123456` 对齐）。

> 说明：本机无 Docker 环境时，仍可按「快速开始」用本地 Go / Node 直接运行；容器化仅提供一种可复现的部署形态。

### 已实现 / 待扩展
| 模块 | 状态 | 说明 |
|------|------|------|
| 用户/角色/菜单/部门（RBAC） | ✅ | JWT + Casbin，按钮级权限，超级管理员豁免 |
| 主机/凭证/分组/授权管理 | ✅ | 凭证 RSA 加密存储，多对多关联；支持禁止 root 直登主机的 sudo 提权（Web Terminal + 任务执行 + 定时任务） |
| Web Terminal | ✅ | xterm.js + WebSocket + SSH PTY，窗口同步，**高危指令实时拦截**（精确/前缀/正则） |
| 会话审计 / 操作日志 | ✅ | 列表/详情/强制终止；录像以 **asciinema v2 格式**落盘（带时间戳），支持在线回放 |
| 高危指令管理 | ✅ | 内置 14 条规则，启停/新增/删除（内置不可删） |
| 脚本库 / 任务模板 / 任务执行 | ✅ | 快速命令/脚本批量并发执行，逐主机结果汇总；**脚本库支持本地文件上传**（按扩展名推断语言，1MB 上限） |
| 定时任务（Cron 调度） | ✅ | 标准 5 段 Cron；命令/脚本定时执行，启停/立即执行/运行状态追踪，启动自动加载；支持「需要 root 执行」开关 |
| 任务执行 / 定时任务「需要 root」开关 | ✅ | 表单勾选后后端自动 `sudo` 切换 root 执行命令（复用主机 sudo 提权，需目标主机已配置可提权凭证） |
| 容器化部署（Docker） | ✅ | docker-compose 一键编排 MySQL+Redis+后端(python3)+前端；`BOAT_` 环境变量覆盖 DB/Redis 主机 |
| IP 白名单（访问控制） | ✅ | 配置 `security.ip-whitelist` 开关 + 列表（支持 CIDR），全局中间件拦截页面与接口，默认关闭 |
| 仪表盘 | ✅ | 资产/审计统计 + 7 天趋势图 |
| SSH 堡垒机（独立 2222 端口） | ✅ | 鉴权用户库、列出授权主机、选主机后带 PTY 反向代理（窗口同步） |
| RDP / VNC / 审批流 | ✅（审批流） | 操作审批流：提交→审批人通过→自动执行并关联交易执行记录；RDP/VNC 见未来规划 |
| MFA 多因素认证 | ✅ | 基于 TOTP 的两步验证：登录二次校验、密钥 RSA 加密存储、认证器扫码绑定、自助启用/关闭 |
| 录像在线回放播放器 | ✅ | 前端 asciinema v2 播放器（xterm 实现，支持暂停/继续/重新播放/倍速），按会话拉取 `.cast` 回放 |
| OSP Agent 执行机管控 | ✅ | 自定义端口 9090 自研加密协议（ECDH P-256 + AES-256-GCM + RSA-PSS 防中间人 + 接入令牌）；`osp-agent` 反向回连常驻，控制台下发命令/脚本任务并回收 stdout/exit code，实时监控节点存活与 CPU/内存/磁盘/负载；断线自动重连，令牌可重置/强制踢下线；覆盖 SSH 不可达时的应急无登录管控 |

### 使用 SSH 堡垒机
后端启动后会监听 `configs/config.yaml` 中 `bastion.port`（默认 2222）。用系统 SSH 客户端连接：
```bash
ssh admin@<服务器IP> -p 2222        # 密码同平台账号（admin / admin123）
```
连接成功后出现主机菜单，输入编号回车即反向代理到目标主机（继承其凭证）。管理员可见全部主机；普通用户仅见「授权管理」中分配给它的主机/分组。

> 说明：堡垒机代理通道当前为全交互透传（目标侧回显），高危指令拦截已在 Web Terminal 通道生效；堡垒机通道的命令级拦截为后续增强项。

### 使用操作审批流
面向高危 / 批量操作提供「申请 → 审批 → 执行」二级闸门，避免越权或误操作直接落地：

1. **提交申请**：在「任务编排 → 操作审批」点击「提交审批」，填写类型（命令 / 脚本）、目标主机、命令或脚本、申请事由。
2. **审批**：具备 `task:approval:approve` 权限的用户（如超级管理员）在审批列表中可见全部待审单；点击「通过」即自动执行并关联交易执行记录，点击「拒绝」需填写理由。申请人不可审批自己的单。
3. **撤回**：申请人可在「待审批」状态下自行撤回。
4. **追溯**：审批单详情展示申请人、事由、审批人、审批意见、执行记录关联，便于审计闭环。

> 权限点：`task:approval:approve`（菜单「操作审批」）。`PUT` 类写操作与执行动作均由后端 `requireApprover` 经 Casbin 二次校验，超级管理员豁免。

---



## 命令行执行npm报错问题
```shell
Set-ExecutionPolicy RemoteSigned -Scope Process

```