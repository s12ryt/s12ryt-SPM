# SPM

以 Go 編寫的 VPS 即時監控系統。主程式接收資料、儲存歷史、判斷告警並提供 Vue 面板；Agent 只採集 Linux／Windows 系統資訊並上報，兩者為獨立執行檔。

- CPU、記憶體、磁碟、網路流量、系統與開機資訊。
- 預設每 3 秒採集，多機搜尋、即時總覽與歷史圖。
- 私人／公開唯讀面板；管理員登入、新增主機、撤銷及輪替 Token。
- SQLite 預設儲存；PostgreSQL URL 設定與重啟自動搬移。
- 全域／單機採樣間隔與告警規則；Web 告警、選用 Webhook／Telegram。

## VPS 一鍵安裝

支援 Linux amd64／arm64 與 systemd。請在 root 終端執行（一般帳號可先 `sudo -i`），需已安裝 `curl`、CA 憑證、`sha256sum` 與 `useradd`。只下載 [Releases](https://github.com/s12ryt/s12ryt-SPM/releases) 已建置的執行檔，不需 Go、Node.js，也不在 VPS 編譯。

預設**只安裝 Server**，初次提示輸入管理員密碼；帳號預設 `admin`，面板為 `http://VPS_IP:8080`：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/s12ryt/s12ryt-SPM/main/install.sh)
```

在面板新增主機並取得 ID／Token 後，到被監控的 VPS 執行，**只安裝 Agent**：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/s12ryt/s12ryt-SPM/main/install.sh) agent
```

初次 Agent 安裝提示主程式 URL、主機 ID、Token；密碼與 Token 不會顯示或記錄在安裝輸出。也可預先 export `SPM_ADMIN_USER`／`SPM_ADMIN_PASSWORD`／`SPM_LISTEN` 或 `SPM_SERVER`／`SPM_NODE_ID`／`SPM_TOKEN` 進行非互動安裝。安裝器的 Server 預設監聽 `0.0.0.0:8080`，可預設 `SPM_LISTEN=127.0.0.1:8080` 搭配 [HTTPS 反向代理](deploy/README.md)。安裝器不修改防火牆。

重跑相同指令更新至最新正式 Release，保留原設定與資料；在最後加 `--version v0.1.0` 可指定版本。腳本先固定版本並校驗 SHA-256，成功後才替換執行檔。服務啟動檢查失敗會嘗試還原舊執行檔與 service 設定，回傳失敗狀態；資料庫回復仍需自己的備份。

- 執行檔：`/opt/spm/spm-server` 或 `/opt/spm/spm-agent`。
- 設定：`/etc/spm/server.env` 或 `/etc/spm/agent.env`，root 擁有、權限 `0600`。更新不覆蓋，修改後重啟對應服務。
- Server 資料：`/var/lib/spm`；服務：`spm-server`／`spm-agent`，以一般 `spm` 帳號執行、開機自動啟動。
- 查看狀態：`systemctl status spm-server`；日誌：`journalctl -u spm-server`（Agent 改用 `spm-agent`）。

`bash (curl ...)` 並非有效 Bash 語法；上述 `<(...)` 會把下載的腳本交給 Bash。

## 建置

需求：Go 1.26 以上、Node.js 22.12 以上及 npm。已在 Go 1.26.3、Node.js 24.11.1 驗證。Node.js 僅供建置前端，正式執行時不需要。

```sh
npm --prefix web ci
npm --prefix web run build
go build -trimpath -ldflags="-s -w" -o bin/spm-server ./cmd/server
go build -trimpath -ldflags="-s -w" -o bin/spm-agent ./cmd/agent
```

Windows 請將輸出名稱改為 `bin/spm-server.exe` 與 `bin/spm-agent.exe`。主程式嵌入 `web/dist`，因此須先建置前端；單獨建置 Agent 不需要前端。

Linux 交叉編譯範例：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/spm-server-linux-amd64 ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/spm-agent-linux-amd64 ./cmd/agent
# ARM64：將 GOARCH 與輸出檔名中的 amd64 改為 arm64。
```

## 快速啟動

Linux 主程式：

```sh
export SPM_ADMIN_USER=admin
read -rs -p '管理員密碼（12–72 bytes）: ' SPM_ADMIN_PASSWORD; echo
export SPM_ADMIN_PASSWORD
./bin/spm-server --listen 127.0.0.1:8080 --data ./data
```

Windows PowerShell 主程式：

```powershell
$env:SPM_ADMIN_USER = 'admin'
$env:SPM_ADMIN_PASSWORD = Read-Host '管理員密碼（12–72 bytes）' -MaskInput
.\bin\spm-server.exe --listen 127.0.0.1:8080 --data .\data
```

開啟 `http://127.0.0.1:8080`，登入後點「新增主機」。記下主機 ID 與只顯示一次的 Token，然後在被監控的主機啟動 Agent：

```sh
export SPM_SERVER=https://monitor.example.com
export SPM_NODE_ID='從面板取得的主機 ID'
read -rs -p 'Agent Token: ' SPM_TOKEN; echo
export SPM_TOKEN
./bin/spm-agent
```

```powershell
$env:SPM_SERVER = 'https://monitor.example.com'
$env:SPM_NODE_ID = '從面板取得的主機 ID'
$env:SPM_TOKEN = Read-Host 'Agent Token' -MaskInput
.\bin\spm-agent.exe
```

本機測試可將 `SPM_SERVER` 設為 `http://127.0.0.1:8080`。跨機部署請使用 HTTPS 反向代理；範例見 [deploy/README.md](deploy/README.md)。程式持續執行，按 Ctrl+C 可結束。Windows Agent 為一般執行檔，可由工作排程器啟動，未實作原生 Windows Service 介面。

## 設定

| 環境變數 | 用途 | 預設 |
| --- | --- | --- |
| `SPM_ADMIN_USER` | 主程式管理員帳號 | 必填 |
| `SPM_ADMIN_PASSWORD` | 管理員密碼，12–72 bytes | 必填 |
| `SPM_LISTEN` | 主程式 HTTP 監聽地址；`--listen` 可覆蓋 | `127.0.0.1:8080` |
| `SPM_DATA_DIR` | SQLite 與資料庫切換設定目錄；`--data` 可覆蓋 | `data` |
| `DATABASE_URL` | 唯一的正式資料庫環境變數，完整 URL | 未設定時使用 SQLite |
| `SPM_SERVER` | Agent 連線的主程式 URL；`--server` 可覆蓋 | 必填 |
| `SPM_NODE_ID` | Agent 主機 ID；`--id` 可覆蓋 | 必填 |
| `SPM_TOKEN` | 該主機的 Agent Token | 必填 |

程式不會自動讀取 `.env`。請由 shell、systemd 或部署工具載入環境變數。修改管理員環境變數後重啟生效；登入 Session 儲存在記憶體中，重啟需重新登入。

Session 有效期為 24 小時；逾期時前端會清除管理資料。前端請求最多等待 10 秒。密碼驗證同時處理一個登入請求，忙碌時回傳 429，稍後可重試。主程式初始化或監聽失敗會以非零狀態結束，供服務管理工具判斷。

### 資料庫

URL 範例：

```text
sqlite:///var/lib/spm/spm.db
sqlite:///F:/SPM/data/spm.db
postgresql://spm:URL編碼後的密碼@db.example.com:5432/spm?sslmode=verify-full
```

PostgreSQL 需預先建立資料庫並給予建表、讀寫權限；表格由主程式建立。密碼中的 `@`、`#` 等字元必須 URL 編碼。SQLite 僅接受絕對路徑，不接受 host 或 query 參數。

從 Web 更換資料庫：

1. 未設定 `DATABASE_URL` 時，登入「系統設定」，填入目標 URL 並儲存。
2. 重啟主程式；主機、Token 雜湊、歷史、設定、告警狀態及通知佇列會複製到目標。
3. 目標必須沒有既有 SPM 資料。目標交易與設定切換都成功後才開始使用新庫，原庫不刪除。
4. 失敗時繼續使用原庫，設定頁顯示錯誤；修正原因後重啟重試，或在設定頁取消待處理搬移。

`DATABASE_URL` 優先於 Web 設定，存在時暫停待處理搬移。直接更換環境變數只會連到指定資料庫，**不會觸發搬移**。`data/database.json` 可能含完整連線憑證，資料庫中也含通知設定，請限制目錄存取權限。搬移需要原庫與目標同時可連線，啟動期間最多等待 30 分鐘。

### 採樣、歷史與告警

預設 CPU／RAM 超過 90% 持續 30 秒、任一磁碟超過 90% 立即告警、15 秒未上報判定離線。實際離線期限至少為 `上報間隔 × 3 + 3 秒`。每台主機可以使用全域規則或完整覆寫，間隔範圍 1–3600 秒，Agent 在下一次成功上報後取得新間隔。

持續超標須由後續樣本確認，不會只靠舊樣本等待時間觸發。CPU 計數不可用、離線／重開機或門檻與持續秒數變更時，尚未成立的相應告警會重新計時；已成立告警會保留到實際恢復。

CPU 與網速由主程式依兩份原始累積計數計算；首次採樣、重開機或計數器重設可能顯示 `—`。網速是非 loopback 介面的總和；虛擬網路／橋接介面可能重複計入流量。部分平台取不到的選用指標會省略。Agent 不執行遠端命令、不保存離線佇列；斷線期間的樣本不補傳。

歷史預設保留 7 天，可調整 1–365 天，每小時清理。CPU、RAM、網速採時間分桶平均，磁碟取桶內最大值；詳情圖最多取 180 個點。告警頁顯示最近 100 筆事件，超過保留時間會清理。每台主機以 3 秒間隔每日約產生 28,800 筆歷史資料，容量需求會隨主機數與保留天數增加。

Web 告警固定提供；Webhook 與 Telegram 在設定頁個別啟用。Webhook 傳送 JSON `{id,nodeId,nodeName,kind,active,time,message}`，`active=false` 表示恢復，`time` 為 Unix 毫秒。Telegram 請填 Bot Token 與 Chat ID。通知每次逾時 5 秒，持久化佇列最多嘗試 5 次並退避；停用管道後會略過該管道待送事件。發送成功但確認前中斷可能造成重送，Webhook 接收端可依 `id` 去重。

公開模式會展示主機名稱、系統、掛載點、流量與歷史，請按需要啟用；通知憑證、資料庫設定及 Agent Token 不會公開。主程式目前適合單一實例執行，同一資料庫不要同時啟動多個主程式。

## 測試

```sh
npm --prefix web test
npm --prefix web run build
go test ./... -count=1
go vet ./...
go test -race ./internal/... -count=1
python scripts/smoke.py                 # Windows 使用 bin/*.exe
python3 scripts/smoke.py --binary-suffix=-linux-amd64 --port=18081
```

Smoke 測試會建立暫存資料庫、啟動主程式與 Agent，驗證真實樣本、登入權限及歷史後自動清理。需先建置對應平台的執行檔；使用 `--serve-seconds 300` 可保留最多 900 秒供瀏覽器驗證。

選用 PostgreSQL 整合測試：在測試環境設定 `TEST_POSTGRES_URL`，執行 `go test ./internal/store -run TestPostgresIntegration -v -count=1`。這個變數僅由測試讀取，需要建立 schema 的權限；測試使用隨機 schema 並於結束時刪除，涵蓋 SQLite → PostgreSQL → SQLite、歷史分桶與通知資料。未提供連線時明確 skip。

初次驗證結果見 [agent/validation.md](agent/validation.md)，後續仔細檢查的修復與最新結果見 [深入稽核紀錄](agent/audit-2026-09-15.md)，需求驗收見 [agent/question.md](agent/question.md)。

## GitHub Linux CI

[Linux CI](https://github.com/s12ryt/s12ryt-SPM/actions/workflows/linux-ci.yml) 在推送 main、Pull Request 或手動觸發時執行。使用 Ubuntu 24.04 runner 與獨立 PostgreSQL 17 service，執行前端測試／型別／建置、Go vet、完整 race（不排除 HTTP 測試）、PostgreSQL 雙向搬移及真實 Linux Agent 上報。CI 拒絕任何略過的測試，並保留測試 JSON 與 Linux 執行檔 14 天；ARM64 產物僅交叉編譯。

[已成功的完整執行](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34950376689)及 [CI 驗證紀錄](agent/ci-validation.md)包含提交版本與實測範圍；原本本機 PostgreSQL／HTTP race 的待驗證項目已由遠端測試補足。

## 自動發布 Releases

[v0.1.0](https://github.com/s12ryt/s12ryt-SPM/releases/tag/v0.1.0) 已發布；[發布與安裝驗收](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34953718316)全部成功，詳見 [Release 驗證紀錄](agent/release-validation.md)。

推送 `vX.Y.Z` 標籤觸發 [Release workflow](.github/workflows/release.yml)。它先重用完整 Linux CI，通過後將該次建置的 Server／Agent（Linux amd64、arm64）及 `SHA256SUMS` 上傳草稿，再發布為正式版本。安裝來源永遠是正式 Release；Artifacts 只用於 workflow 內傳遞已驗證產物。

發布後以乾淨 Ubuntu runner 從公開 Release 真正安裝 systemd 服務，檢查 Server／Agent 角色隔離、特殊字元密碼、實際採樣與更新保留資料；失敗會把該 Release 撤回草稿。標籤應指向 main 已審查的提交，例如 `git tag v0.1.0` 後 `git push origin v0.1.0`。版本資產不覆寫；有修正時發布新版本。
