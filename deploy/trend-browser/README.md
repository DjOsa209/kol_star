# 可选旧方案：Linux 服务器浏览器

**当前主方案已经改为 [本地采集上传](LOCAL-UPLOAD.md)**，服务器不需要 Chrome 或远程桌面。下面仅保留此前的服务器浏览器方案，默认不启用。

这是服务器的 **attach** 部署方案。普通 Chrome 与后台运行在同一台 Linux 主机，用户通过独立认证的 HTTPS 远程桌面手动登录，随后采集程序连接现有 Chrome。无需浏览器扩展，不复制本地电脑的 Cookie 或 profile。

本仓库已完成本地 Mac 的会话复用和 Tag / Video 各 100 条及详情写入 Redis 验证。Linux 服务、远程桌面及目标服务器的 Google 登录尚未实测，由部署方验证。旧 `kol-admin-trend-browser.service.example` 是自动启动浏览器的 launch 方案，本次不要使用它。

## 1. 准备环境

- 同机后台和 Nginx，后台公网入口使用有效 HTTPS；现有 MySQL、Redis 配置继续使用。
- 两个普通系统用户：后台 `kol-admin`，桌面 `kol-trend-browser`。桌面用户不得有权读取后台配置、应用凭据或其他用户文件。
- 运维从批准的供应商或发行版软件源预先安装：Chrome 144+、Bash 4.3+、Xvfb、xauth、x11vnc、openbox、websockify、noVNC、flock、GNU stat。脚本不下载、安装、升级任何包。
- 按组织审批流程选定版本、检查许可证及安全公告，固定软件源快照或包版本并记录安装清单；不能把未审计的最新版本当作已验收依赖。本项目不新增 npm / Go 依赖。
- Chrome 访问 Ads / Google 所需的网络由服务器环境提供，不关闭 TLS 校验、sandbox，也不添加隐藏自动化的参数。

源码依据：[Chrome 现有会话连接](https://developer.chrome.com/blog/chrome-devtools-mcp-debug-your-browser-session)、[noVNC](https://github.com/novnc/noVNC)、[websockify](https://github.com/novnc/websockify)、[Nginx WebSocket 代理](https://nginx.org/en/docs/http/websocket.html)。noVNC 的主要许可证为 MPL-2.0，websockify 为 LGPL-3.0；具体安装包及附带组件按发行版清单核对。

## 2. 文件与配置

以下以 `/opt/kol-admin` 为项目目录，路径和系统账号不同则同步修改两个 service 模板。

1. 按项目现有流程构建前端和后端，后端二进制放在 `/opt/kol-admin/backend/kol-admin-server`，前端部署到原后台站点。
2. 将 `collector.conf.example` 复制为同目录的 `collector.conf`，填写 `TREND_BROWSER_ATTACHED_USER_ID` 为允许使用该采集浏览器的后台用户数字 ID。不是 Google 用户名。该文件仅存非敏感启动配置，不填写 Google 密码。
3. 准备 `/var/lib/kol-trend-browser`，所有者 `kol-trend-browser`，权限 `0700`。Chrome profile 持久保存在该目录；不要每次发版清空。
4. 用桌面用户交互运行 `x11vnc -storepasswd /var/lib/kol-trend-browser/vnc.pass`，在服务器终端设置远程桌面密码。文件必须为该用户所有、权限 `0600`。不要把密码放入命令参数、仓库或聊天。
5. 为 Nginx 独立创建 `/etc/nginx/trend-desktop.htpasswd`，仅让 Nginx 服务账号可读。通过运维的交互密码工具配置，不在模板中硬编码。
6. 后台自己的敏感配置只允许后台系统用户读取。桌面用户可以操作其整个桌面，因此远程桌面两层凭据只交给 `attached_user_id` 对应的操作者；后台菜单鉴权不能代替这个远程桌面网关的认证。

默认端口：VNC `127.0.0.1:5901`，noVNC/WebSocket `127.0.0.1:6080`，Chrome 调试 `127.0.0.1:9222`，后台 `127.0.0.1:8080`。不向外网发布这些端口。桌面和后台需共享主机回环网络；本模板不是跨容器连接方案。

## 3. 接入原后台域名

将 `nginx-desktop.conf.example` 内容合入现有后台的 **HTTPS server 块**。它为同域 `/trend-desktop/` 提供 Basic Auth、WebSocket 转发、跨来源请求拦截和不缓存策略，缺少密码文件会拒绝访问。不要绕过该 location 直接提供 noVNC。

先由运维执行 `nginx -t`，通过后按既有发布流程重载。未登录访问 `/trend-desktop/vnc.html` 应得到 401；普通 HTTP 不得呈现桌面。确认认证后静态页面、JavaScript、CSS、WebSocket 均能访问，且 WebSocket 返回 101。

## 4. 启动两个服务

将两个新模板分别安装为：

- `kol-admin-trend-desktop.service`：普通 Chrome、虚拟显示、窗口管理器、VNC 与 noVNC。
- `kol-admin-trend-attach.service`：现有业务后端，连接上述 Chrome；应替换当前后端服务，不能并行占用 8080。

在桌面用户身份下运行 `scripts/start-trend-linux-desktop.sh --check` 可检查依赖和目录权限，不启动服务。实际启动使用 systemd 加载 `collector.conf`，然后由运维按正常发布流程启动桌面及后端。不需要让后端服务启动 Chrome。

桌面组件退出会整体清理该实例并由 systemd 重启；正常退出给 Chrome 留出保存 profile 的时间。只重启后端不会结束桌面服务。服务日志只报告固定状态，浏览器与 VNC 的原始输出不写应用日志，避免记录登录地址或输入。依赖问题可先用 `--check` 和服务退出状态定位。

## 5. 首次登录与后续使用

1. 在后台“资源运营 → 趋势洞察”点击“读取最新”。第一次 Chrome 尚未开启调试时，连接失败是预期状态；对话框会显示“打开服务器浏览器”。
2. 点击入口，完成网关认证和 VNC 认证。在服务器 Chrome 中手动打开 Ads 并完成 Google 登录。此时没有采集调试连接。
3. 在这个 Chrome 打开 `chrome://inspect/#remote-debugging`，启用远程调试。回后台点击“重新采集”，在远程桌面里的 Chrome 允许调试连接。确认实际端点与 `TREND_BROWSER_ENDPOINT` 一致。
4. 登录有效时直接采集并更新 Redis，后续查询复用连接。会话失效时后台断开控制、保留 Ads 页面；点击“打开服务器浏览器”登录，再点“登录完成，继续采集”。
5. Chrome 可能再次询问调试授权，这不等于重新登录 Google。浏览器重启后是否需要重新授权，以 Chrome 当前提示为准。

服务器桌面按钮只在 `trend_browser.desktop_enabled` / `TREND_BROWSER_DESKTOP_ENABLED` 开启后显示。尚未完成 HTTPS、网关认证和 VNC 认证时保持关闭。窗口受浏览器弹窗策略阻止时，允许原后台域名打开窗口后重试。

## 6. 现场验收与回退

- 首次 Google 登录成功，Ads 原站能翻到第二页。
- Tag / Video 分别读取 US / 30 天，完整 100 条及详情更新，重新打开最新数据页仍可读。
- 重复读取不重新登录；后端重启不关闭 Chrome；Chrome 正常重开后检查会话能否保留。
- 手动退出 Ads 后提示登录；失败或取消不覆盖旧 Redis 结果；重新登录后继续成功。
- 不同后台用户无法调用这名操作者的采集任务；未认证的远程桌面请求被拒绝。
- 日志不包含密码、Cookie、认证参数或登录画面。

Google 是否接受该服务器 IP 和 Linux 浏览器需要现场验证，接入完成不代表这项验证已通过。出现“不安全浏览器”或验证码时由操作者处理，不降低浏览器安全设置。

若接入失败，停用新 attach 后端服务，按原发布流程恢复旧后端并关闭 `desktop_enabled`；停止桌面服务及撤下 Nginx location。保留私有 profile 目录，不删除登录状态或原有业务缓存。
