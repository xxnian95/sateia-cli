# Sateia CLI（简体中文）

[English](README.md) | 简体中文

`sateia` 用于读取和创建 Sateia 服务端的营养记录。CLI 创建的记录会同步到
Sateia App。CLI 支持交互式登录、自动化场景的托管 token、单页查询、机器可读
输出，以及创建记录失败后的幂等重试。

当前 CLI 只支持查询和创建记录，不支持编辑或删除记录。

## 安装

需要 Go 1.26 或更高版本。

```sh
go install github.com/xxnian95/sateia-cli/cmd/sateia@latest
sateia --version
```

如果第二条命令找不到 `sateia`，请将 Go 的二进制目录加入 `PATH` 后重试。

## 快速开始

1. 运行 `hostname`，为当前机器选择一个稳定且可识别的名称，例如
   `agent-host-01 (Sateia CLI)`。
2. 在 Sateia App 中打开 **设置 > CLI Access**，创建一个一次性代码。
3. 使用代码换取 token 并保存：

   ```sh
   sateia auth login \
     --device-code ABCD-EFGH \
     --device-name "agent-host-01 (Sateia CLI)"
   ```

4. 验证保存的凭证：

   ```sh
   sateia auth status
   ```

5. 查看要执行的操作：

   ```sh
   sateia record list --help
   sateia record create --help
   ```

一次性代码的有效期为五分钟，并且只能兑换一次。设备名称由 CLI 作为 token
元数据提交，不需要与 App 以前显示的标签一致。

## 身份认证

### 凭证来源

CLI 每次执行时按以下顺序选择一个凭证：

| 优先级 | 来源 | 持久化方式 | 适用场景 |
| --- | --- | --- | --- |
| 1 | `--token` | 不持久化 | 单次命令 |
| 2 | `SATEIA_TOKEN` | 由环境管理 | 使用环境变量托管密钥的自动化任务 |
| 3 | `SATEIA_TOKEN_FILE` | 作为密钥文件管理；`auth login --token-file` 可以创建 | 容器、Agent 和挂载密钥 |
| 4 | 系统凭证库 | 由 `auth login` 保存 | 交互式机器 |

`auth status` 会显示 `credential_source`，但不会输出 token。

命令行 token 可能出现在 shell 历史或进程列表中。自动化场景应优先使用
`SATEIA_TOKEN` 或 `SATEIA_TOKEN_FILE`。只在本次命令中使用 token：

```sh
sateia --token "$TOKEN" auth status
```

### 无桌面环境的 Linux

Linux 系统凭证库依赖 Secret Service 和用户 D-Bus 会话。如果机器没有这些
服务，可以在兑换设备代码时创建一个新的私有 token 文件：

```sh
sateia auth login \
  --device-code ABCD-EFGH \
  --device-name "agent-host-01 (Sateia CLI)" \
  --token-file "$HOME/.config/sateia/token"

export SATEIA_TOKEN_FILE="$HOME/.config/sateia/token"
sateia auth status
```

父目录必须已经存在。CLI 会以 `0600` 权限创建文件，并拒绝覆盖已有路径。CLI
会先占用文件路径，再消费只能使用一次的代码。

### 登出与撤销

```sh
sateia auth logout
```

`auth logout` 只删除系统凭证库中的 token，不会删除由 `--token`、
`SATEIA_TOKEN` 或 `SATEIA_TOKEN_FILE` 提供的凭证，也不会撤销服务端 token。
环境变量和文件中的密钥需要在其来源处删除。需要让 token 在所有位置失效时，
请在 **Sateia App > 设置 > CLI Access** 中撤销它。

## 查询记录

CLI 在明确的摄入时间范围内返回一页记录：

```sh
sateia record list \
  --consumed-from 2026-08-01T00:00:00+08:00 \
  --consumed-before 2026-08-08T00:00:00+08:00 \
  --limit 50 \
  --json
```

`--consumed-from` 是包含边界，`--consumed-before` 是不包含边界，两者都使用
RFC 3339 时间。`--limit` 必须在 1 到 100 之间。结果按时间从新到旧排列；除非
指定 `--include-deleted`，否则不返回已删除记录。

如果 `has_more` 为 `true`，使用完全相同的时间范围、`--include-deleted` 选择和
limit 重复命令，并将 `next_cursor` 作为 `--cursor` 传入。cursor 是与完整过滤条件
绑定的不透明值，不要修改或解码。需要从第一页重新开始时，省略 `--cursor`。

## 创建记录

创建记录会写入服务端数据：

```sh
sateia record create \
  --energy 520 \
  --protein 28.5 \
  --carbohydrate 62 \
  --fat 18 \
  --note "午餐" \
  --consumed-at 2026-08-07T12:30:00+08:00 \
  --json
```

能量单位是千卡，蛋白质、碳水化合物和脂肪单位是克。四个值都是必填项，必须
使用非负十进制数，最多包含六位小数。`--consumed-at` 接受带明确 UTC 偏移的
RFC 3339 时间；省略时使用当前时间。

CLI 会生成 `record_id` 和 `mutation_id`。如果创建请求遇到结果不确定的网络错误
或临时服务错误，错误信息会输出这两个值。重试时必须保持原始请求内容不变，
并传入输出的 `--record-id` 和 `--mutation-id`。复用 `mutation_id` 却修改请求内容
会触发幂等冲突；生成新的标识符则可能创建重复记录。

## 机器可读输出

当其他程序或 AI agent 需要消费结果时，请使用 `--json`。成功的 JSON 响应包含
顶层 `_notice` 数组：

- `NEXT_PAGE` 表示还有下一页查询结果。
- `UPDATE_AVAILABLE` 包含安装最新稳定版 CLI 的完整命令。

服务端提供请求标识时，响应还会包含 `request_id`。诊断错误时应报告该值，以便
运维人员关联服务端日志和审计事件。它是请求元数据，不是记录标识符或 cursor。

CLI 会在成功执行命令后检查更新，并缓存结果 24 小时。更新检查失败不会改变
当前命令的结果。在隔离网络环境中，可以设置 `SATEIA_NO_UPDATE_NOTIFIER=1`
禁用检查。

## AI agent 使用规则

实时帮助是命令契约。AI agent 应当：

1. 第一次操作前运行 `sateia --help` 和 `sateia environment`。
2. 使用设备代码登录前运行 `hostname`，并提出稳定的当前机器名称。
3. 不输出 token，也不把 token 写入日志、仓库文件或消息；只使用选定的密钥管理
   环境或 token 文件。
4. 查询时使用明确的时间范围；翻页时保持所有过滤条件不变。
5. 写入前运行 `sateia auth status`。
6. 只有用户明确要求写入时才创建记录，不猜测缺失的营养值，也不把已知摄入
   时间替换成当前时间。
7. 优先使用 `--json`；只有在退出状态为零且成功响应可以解析时才报告成功。
8. 对结果不确定的创建请求进行重试时，保留两个标识符和完全相同的请求内容。

仓库还提供了 Agent Skill：
[`skills/use-sateia-cli/SKILL.md`](skills/use-sateia-cli/SKILL.md)。如果已安装 CLI
的实时帮助与 Skill 不一致，以实时帮助为准。

## 故障排查

- **未找到凭证：**运行 `sateia auth login`，传入 `--token`，或者设置
  `SATEIA_TOKEN` 或 `SATEIA_TOKEN_FILE`。
- **Linux 系统凭证库不可用：**使用托管 token、执行
  `auth login --token-file <new-path>`，或者启动 Secret Service 和用户 D-Bus
  会话。该错误不能证明系统凭证库中不存在 token。
- **`UNAUTHENTICATED`：**运行 `sateia auth status`。错误信息会根据当前使用的
  argument、environment、token_file 或 keyring 来源给出替换步骤。
- **`INVALID_CURSOR`：**恢复生成该 cursor 时使用的所有过滤条件，或者省略
  `--cursor` 从头开始。不要修改 cursor。
- **创建结果不确定：**使用错误信息输出的 `--record-id` 和 `--mutation-id`，
  保持原请求内容不变后重试。
- **服务端错误包含 `request_id`：**请求运维人员排查日志时附上该值。

运行 `sateia environment` 可以查看完整的凭证、恢复、输出和 Agent 安全指南。

## 服务端与本地配置

CLI 按以下顺序选择服务端：

1. `--server`
2. `SATEIA_SERVER`
3. `auth login` 保存的服务端
4. `https://xxnian.site/sateia-server`

远程服务端必须使用 HTTPS；只有回环地址可以使用 HTTP。`SATEIA_CONFIG_DIR`
用于改变非敏感配置和更新检查缓存的存储位置。token 不会写入 `config.json`。

## 许可证

MIT
