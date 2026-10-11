# 遠端專案問答：A 的 pi agent，B 的 MCP client

第一版發布單一專案。B 送出問題，A 的 pi agent 搜尋 aascribe 索引、讀取來源，回傳答案、相對路徑、行號與 SHA-256。A 的 IP 可以改變：用固定網域的 Cloudflare Tunnel，B 始終連同一個 HTTPS `/mcp` 網址。

```text
B 的 AI agent
  │ HTTPS MCP: ask_project({question}) + Bearer Token
  ▼
固定網域 / Cloudflare Tunnel
  │ A 主動建立連線；本機轉送至 127.0.0.1:8080
  ▼
A: aascribe-mcp
  │ 每個問題啟動一個 Node 子程序與獨立 pi session
  ▼
pi SDK → 私有 loopback 工具 bridge → aascribe 索引與已發布來源
  │ 搜尋、閱讀、產生答案
  ▼
Go 驗證引用確實來自讀過的來源，再回傳 B
```

## A：安裝與準備

需要 Go 1.26、Node.js 22.19.0 或更新版本、npm，以及 pi 使用的模型 API key。這裡固定使用 `@earendil-works/pi-coding-agent@1.1.0`，不必全域安裝 pi CLI。Go 呼叫獨立 Node worker，worker 用 pi SDK。

在此 repository 根目錄執行（PowerShell）：

```powershell
npm.cmd ci --prefix runtime/pi --ignore-scripts
go build -o dist/aascribe.exe ./cmd/aascribe
go build -o dist/aascribe-mcp.exe ./cmd/aascribe-mcp
```

準備一個只包含願意發布內容的專案目錄，例如 `C:\knowledge\my-project`。建議使用乾淨的 checkout 或匯出的文件目錄，避免把整個使用者目錄作為 root。檔案內容將由 A 傳給所選模型供應商；B 會收到答案及引用。

可以先使用既有 `aascribe index` 產生索引，也可以直接啟動 MCP，再由 B 呼叫 `index_project` 建立第一份索引：

```powershell
# GEMINI_API_KEY 從環境或你的 secret 管理方式取得，不要放入 Git。
.\dist\aascribe.exe index C:\knowledge\my-project --depth 20 --max-file-size 1048576
```

此命令可沿用既有索引設定。MCP 查詢的 pi 模型則使用下方的 `--pi-provider`、`--pi-model`，不讀取 aascribe 的 `config.toml`。若 index 使用的不是預設 Gemini，依 [configuration.md](configuration.md) 配置。

只發布已有摘要、`status=ok`、所屬 metadata 未標為 dirty、大小不超過 1 MiB 的一般 UTF-8 檔案。`--no-summary` 的索引不會成為問答來源。預設排除 `.env*`、`.git`、`.aws`、`.ssh`、`.codex`、`.pi`、`node_modules`、`.venv`、`dist`、`target`；最多 10,000 個來源。這不是通用的秘密偵測功能，發布內容仍由 A 的目錄管理者決定。舊 metadata 可能仍存在於曾經索引但後來排除的目錄，因此應在乾淨的發布目錄建立索引。

## A：啟動 MCP

第一次產生 256-bit 隨機 Token（不要輸出到共享日誌）：

```powershell
$tokenBytes = New-Object byte[] 32
$tokenRng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$tokenRng.GetBytes($tokenBytes)
$tokenRng.Dispose()
$env:AASCRIBE_MCP_TOKEN = [BitConverter]::ToString($tokenBytes).Replace('-', '').ToLowerInvariant()
```

把同一個 Token 安全地配置到 B 的 MCP client。正式執行時，A 和 B 都應從 secret 管理方式載入這個固定 Token；每次重啟重新產生會使 B 的舊 Token 失效。更換 Token 後重啟 A。

```powershell
# 預設 google / gemini-2.5-flash 使用 GEMINI_API_KEY。
# 或以 AASCRIBE_PI_API_KEY 覆蓋所選 provider 的 key。
.\dist\aascribe-mcp.exe `
  --root C:\knowledge\my-project `
  --pi-worker .\runtime\pi\worker.mjs `
  --pi-provider google `
  --pi-model gemini-2.5-flash `
  --query-timeout 90s
```

保持此程序在線。預設監聽 `127.0.0.1:8080`，適合 Tunnel。非 loopback 的 listener 必須同時指定 `--tls-cert` 與 `--tls-key`。

| 選項 | 預設 | 用途 |
|---|---|---|
| `--root` | 必填 | 單一發布目錄 |
| `--listen` | `127.0.0.1:8080` | MCP listener |
| `--node` | `node` | Node executable |
| `--pi-worker` | `runtime/pi/worker.mjs` | worker 路徑，啟動時轉為絕對路徑 |
| `--pi-provider` | `google` | pi model provider |
| `--pi-model` | `gemini-2.5-flash` | pi model ID |
| `--pi-models-file` | 無 | 明確指定 pi 的自訂 models.json |
| `--query-timeout` | `120s` | 單次逾時，1 秒到 5 分鐘；Tunnel 範例用 90 秒 |
| `--query-concurrency` | `1` | 同時執行 1–8 個 pi worker；超過立即拒絕 |
| `--origins` | 無 | 允許的精確 Origin，逗號分隔；沒有 Origin 的 native client 可連線 |

Token 至少 32 字元且不能含空白。所有 `/mcp` 請求必須帶 `Authorization: Bearer <token>`。本版是預先配置 Token 的 native MCP client 整合，不含 OAuth 登入流程或 browser CORS；需要 OAuth-only connector 的 client 無法直接使用。

自訂 `models.json` 是 A 的可信任管理者設定，支援內容以 pi 的已安裝版本文件為準。pi 的設定可包含 credential command，因此不能允許 B 上傳或選擇此設定檔。

## B：AI agent 的 MCP 連線

在支援 Streamable HTTP 與自訂 Authorization header 的 MCP client 配置：

| 欄位 | 值 |
|---|---|
| Server 名稱 | `aascribe-project` |
| Transport | Streamable HTTP |
| URL | `https://knowledge.example.com/mcp` |
| Header | `Authorization: Bearer <A 與 B 共用的 Token>` |
| Client request timeout | 大於 A 的 query timeout，例如 110 秒對應 A 的 90 秒 |

各 client 的設定檔語法不同；使用它的 secret/env 機制保存 Token。問答工具：

```json
{"name":"ask_project","arguments":{"question":"登入流程如何驗證 Token？請附來源。"}}
```

### 遠端建立或更新索引

B 可以呼叫 `index_project`，範圍固定為 A 啟動時的 `--root`。工具不接受遠端路徑，也不修改來源檔案；它會取代由 aascribe 管理的索引 metadata，成功後立即載入給 `ask_project` 使用。

```json
{"name":"index_project","arguments":{}}
```

預設重新掃描專案、更新變動檔案，重用未變動檔案的摘要；若要重新建立全部摘要：

```json
{"name":"index_project","arguments":{"refresh":true,"depth":20}}
```

`depth` 可為 0–64，預設 20；0 只索引 root 的直接檔案。工具使用現有 index 引擎的本地擷取式摘要，不呼叫模型；既有未變動的 AI 摘要會保留，`refresh=true` 則改用本地摘要重建。遵守 `.gitignore`、`.aaignore` 與預設排除規則，略過 symlink，檔案上限 1 MiB；同一次掃描中排除或刪除的來源不再發布。

成功結果包含 `status`（`completed` 或 `completed_with_warnings`）、`published_files`、`refresh`、`depth`、`duration_ms`、`summary_mode` 與 `notice`。部分檔案失敗時可完成其餘檔案，失敗來源會省略。

索引與問答互斥：有問答執行中，索引會回覆 busy；索引執行中，問答及其他索引也會回覆 busy，請稍後重試。套用相同的 `--query-timeout`。取消或失敗可能已寫入部分資料夾的 metadata，但不會替換目前記憶體中的 catalog；重新呼叫工具完成更新即可。所有持有此 MCP Token 的 client 都能呼叫此工具。

MCP `tools/call` 的 `result.structuredContent` 包含：

```json
{
  "status": "answered",
  "answer": "根據來源，Access Token 在 30 分鐘後到期。",
  "sources": [
    {"path":"auth.md","start_line":2,"end_line":2,"hash":"sha256:..."}
  ],
  "provider":"google",
  "model":"gemini-2.5-flash",
  "usage":{"turns":3,"total_tokens":2151,"estimated_cost_usd":0.001},
  "notice":"..."
}
```

這是格式示例，並非效能或費用保證。證據不足時回傳 `status=insufficient_evidence`，`sources` 可以為空。模型、逾時、容量與引用驗證失敗會回傳 MCP tool error。

不需要 AI client 就能先測試整條 MCP 連線：

```powershell
# B 已透過你的 secret 管理方式配置 AASCRIBE_MCP_TOKEN。
.\scripts\ask-project.ps1 `
  -Url https://knowledge.example.com/mcp `
  -Question '登入流程如何驗證 Token？'
```

在 A 本機先把 URL 換成 `http://127.0.0.1:8080/mcp`。腳本會完成 initialize、tools/list、tools/call，不只是檢查 HTTP 存活。

## 動態 IP：固定網域的 Cloudflare Tunnel

需要 Cloudflare 帳號及已加入該帳號的網域，例如 `example.com`。使用有固定網域的 named/managed tunnel，避免把隨機網址的 Quick Tunnel 當作正式部署。

1. 在 Cloudflare dashboard 的 **Networking → Tunnels** 建立 Tunnel，例如 `aascribe-a`。
2. 選擇 A 的作業系統，在 A 執行 dashboard 提供的 cloudflared 安裝與連線指令。Windows 可依 dashboard 指示安裝為服務。
3. Tunnel 顯示 Healthy 後，新增 **Published application route**：hostname `knowledge.example.com`，service URL `http://127.0.0.1:8080`。
4. 在該 route 的 origin 額外設定，把 **HTTP Host Header** 設為 `localhost:8080`。這是必要設定：官方 MCP SDK 預設拒絕 loopback listener 上的非 localhost Host，不能為了 Tunnel 關掉這項保護。路徑 `/mcp` 應原樣保留。
5. 保留 Authorization header 轉送；不要對 `/mcp` 加入需要瀏覽器互動的 challenge/login。若再加 Cloudflare Access，B 的 client 必須能傳送 Access service-token headers，且仍需 aascribe 的 Bearer Token。
6. 在 B 用上面的 smoke script 驗證固定 HTTPS 網址。

Cloudflared 主動連往 Cloudflare，不需要路由器 port forwarding，也不依賴 A 的固定公開 IP。IP 變更後，Tunnel 可重建連線；正在執行的請求仍可能中斷，需要 B 明確重試，而重試可能再產生模型費用。A 休眠、離線或服務未啟動時，無法回答。

正式使用可把 cloudflared 安裝為 Windows service；aascribe-mcp 用 Task Scheduler 在登入/開機後啟動並於失敗時重啟。設定工作目錄、絕對的 binary/root/worker 路徑，並讓該帳號載入 Token 與模型 key。安裝範例保留前景程序，沒有替你變更系統服務、DNS 或 Cloudflare 帳號。

參考官方文件：[Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/)、[Dashboard 建立方式](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel/)、[Origin parameters](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/origin-parameters/)。

## 執行邊界與更新

每個問題只有一個獨立 Node 子程序與記憶體內的 pi session；不沿用其他問題的聊天歷史，不載入 project/global 的 skills、extensions、context files 或 shell 工具。pi 僅使用 Go bridge 提供的 `knowledge_list`、`knowledge_search`、`knowledge_read`。來源讀取使用 Go `os.Root` 限制在發布 root，並核對索引中的 SHA-256。

每次最多 8 個模型請求、每次模型輸出最多 4,096 tokens、24 次知識工具呼叫、128 KiB 序列化工具上下文、60,000 provider-reported 累積 tokens。Token 上限在每次模型回應後檢查，因此最後一個回應可以超過門檻；這不是供應商帳單的硬額度。請另設模型帳戶的費用限制。

回傳引用必須落在 pi 成功讀過的來源行範圍內，hash 由 Go 補上；回傳前再次確認來源未改變。這能阻止虛構路徑與未讀來源引用，但不能證明答案中的每一句推論都正確。

索引在 MCP 啟動時載入。檔案改變會使讀取失敗，摘要仍可能是舊快照。更新內容後由 B 呼叫 `index_project` 即可重新掃描及載入，不需重啟 MCP。A 的 pi 問答 session 保持唯讀；索引工具由 Go 執行。若是在 A 直接跑本地 `aascribe index`，也可再呼叫遠端 `index_project` 載入，或重啟 MCP。

## 測試與驗證

### Windows 背景啟動與完整 MCP 檢查

建置 `dist/aascribe-mcp.exe` 並安裝 pi 依賴後，在 A 執行：

```powershell
./scripts/start-mcp.ps1
```

預設發布這個 repository，只監聽 `127.0.0.1:8080`。也可用 `-ProjectRoot` 指定發布目錄。背景啟動腳本預設使用 `openai-codex` 的 `gpt-6.1-sol`，沿用目前 Windows 帳號的 Codex-managed ChatGPT 訂閱登入。腳本產生固定 MCP Token，並用 Windows DPAPI 加密保存於 Git 忽略的 `data/remote-mcp/credentials.dpapi`，僅供同一 Windows 使用者解密。後續啟動沿用 Token。PID 與每次啟動的 log 保存在相同目錄；這個背景程序尚未設定開機自動啟動。

```powershell
codex login status
# 尚未登入時，使用 codex login 登入擁有訂閱的帳號。
./scripts/start-mcp.ps1
```

預設模式從明確指定的 `~/.codex/auth.json` 讀取有效的 access token，只放在 pi worker 記憶體，不複製或保存 refresh token。快到期時，只透過官方 Codex app-server 的 `account/read` 更新登入，不啟動 Codex inference 或工具執行；問答與來源工具仍由 pi 運行。這個模式需要 Codex CLI 與檔案型登入儲存，僅有 OS credential store 時應改用專用 pi 登入。獨立 server CLI 以 `--pi-provider openai-codex --pi-model gpt-6.1-sol --pi-codex-auth-file <絕對路徑> --codex-command <Codex執行檔>` 啟用。登入失效就失敗，不改用 API key。

如需讓服務使用獨立登入，執行 `node runtime/pi/login-codex.mjs`，依印出的網址完成登入，再用 `./scripts/start-mcp.ps1 -AuthFile <data/remote-mcp/pi/auth.json 的絕對路徑>`。獨立 server CLI 對應 `--pi-auth-file`，與 `--pi-codex-auth-file` 互斥。專用 OAuth 憑證是敏感的明文 Token 檔，應限制目錄權限至執行服務的帳號，保持該目錄未被索引與發布。供應商回報的 `estimated_cost_usd` 是模型目錄估計值，不代表訂閱扣款。若明確選用 Gemini，才使用 `./scripts/start-mcp.ps1 -Provider google` 與 `GEMINI_API_KEY`。

### 沒有自有網域：ngrok 免費固定開發網域

ngrok Free 配發一個帳號固定的開發網域，不需要買網域；A 的 IP 或程序重啟後仍可使用同一網址。登入 ngrok Free 帳號，把官方執行檔放在 `data/remote-mcp/ngrok/ngrok.exe`，在 A 本機安全設定 `NGROK_AUTHTOKEN`，再執行：

```powershell
./scripts/start-ngrok.ps1
# 若已取得帳號配發的固定網址，也可明確指定：
./scripts/start-ngrok.ps1 -PublicUrl https://your-assigned-name.ngrok-free.app
```

腳本將 ngrok Token 用 Windows DPAPI 保存，背景程序重寫 Host 為 `localhost:8080` 並關閉 HTTP inspection。`--host-header` 目前仍受支援但官方標示已 deprecated，升級 ngrok 前應再檢查 CLI。不要把 ngrok 的 authtoken 交給 B；B 只需要 MCP Bearer Token 與公開 `/mcp` URL。API client 不受免費方案的 HTML 瀏覽器提示頁影響。目前免費額度為每月 1 GB 出站流量及 20,000 HTTP 請求，MCP 初始化與通知也算 HTTP 請求；超額會影響可用性，不會由這些腳本自動升級方案。參考 [Free Plan Limits](https://ngrok.com/docs/pricing-limits/free-plan-limits) 與 [ngrok CLI](https://ngrok.com/docs/agent/cli)。

在 A 的同一 Windows 帳號載入 client Token（不要把解密結果輸出到聊天或 log）：

```powershell
$secure = (Get-Content data/remote-mcp/credentials.dpapi -Raw).Trim() | ConvertTo-SecureString
$saved = ([System.Net.NetworkCredential]::new('', $secure)).Password | ConvertFrom-Json
$env:AASCRIBE_MCP_TOKEN = $saved.token
# 純本機建立/更新索引，不呼叫模型：
./scripts/ask-project.ps1 -Url http://127.0.0.1:8080/mcp -IndexOnly
# 建立/更新索引後進行真實 pi 問答（來源片段送至設定的模型供應商）：
./scripts/ask-project.ps1 -Url http://127.0.0.1:8080/mcp -IndexFirst -Question 'Explain ask_project and cite implementation files.'
```

固定網域上線後，在 B 安全設定相同的 `AASCRIBE_MCP_TOKEN`，以 `https://你的固定網域/mcp` 替換 URL 執行 `-IndexFirst -Question ...`。`-Refresh` 強制重新產生摘要；未指定時沿用未變更的索引。腳本驗證 initialize、工具發現、index_project 與 ask_project，不只測 HTTP 連通。A 上的外部 URL 測試與 B 實機測試是不同的驗證，B 的測試必須從 B 執行。

```powershell
npm.cmd test --prefix runtime/pi
go test ./internal/knowledge ./internal/projectask ./internal/mcpserver ./cmd/aascribe-mcp

# 使用現有 GEMINI_API_KEY 對合成專案查詢；會呼叫模型並產生費用。
$env:AASCRIBE_LIVE_PI_TEST = '1'
go test ./internal/mcpserver -run TestLivePiQuery -count=1 -v
Remove-Item Env:AASCRIBE_LIVE_PI_TEST
```

一般整合測試使用真正的 MCP client/server、真正的 pi SDK 與工具迴圈，只有模型供應商以本機 SSE fixture 替換；可重現且不需模型 key。測試涵蓋 Token/Origin、UTF-8 分頁與行號、來源變更、未發布來源、引用驗證、輸出限制、逾時、取消、容量回收，以及遠端建立/更新索引、即時載入、刪除/排除來源和問答互斥。無 Node 或未安裝 pi 依賴時，Go 的 pi SDK 整合測試會明確 skip；請先跑 `npm ci`。Windows 無建立 symlink 權限時，外部 symlink 測試會 skip。

目前 repository 的完整 `go test ./...` 另有既有 `tests/index-fixtures` 缺少造成的失敗；新增問答套件的測試與既有 CLI 的測試應分別檢查。Tunnel 步驟依官方文件整理，尚未以你的 Cloudflare 帳號及實際網域部署驗證。

### 此 A 的部署狀態與 B 設定

已啟動 A 的背景 MCP 與 ngrok，固定端點為 `https://condimental-knurliest-dagny.ngrok-free.dev/mcp`。公開初始化、工具發現、索引更新，以及缺少或錯誤 Bearer 回傳 401 已驗證。本機 Codex 訂閱問答已成功；公開真實問答曾失敗，仍待重測，不能視為完整通過。未從 B 實機測試。

B 使用 Codex 時，將 `scripts/b-machine-mcp.toml` 合併到 B 的 `~/.codex/config.toml`，在 B 安全設定 `AASCRIBE_MCP_TOKEN` 後啟動 agent。只能交付 MCP Bearer，不能交付 ngrok 或 Codex 憑證。先請 agent 呼叫 `index_project`，再呼叫 `ask_project`。A 重開機後需先執行 `scripts/start-mcp.ps1`，再以相同固定網址執行 `scripts/start-ngrok.ps1 -PublicUrl https://condimental-knurliest-dagny.ngrok-free.dev`。