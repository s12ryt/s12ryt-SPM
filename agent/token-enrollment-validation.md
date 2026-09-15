# Agent 免填 ID 驗證

日期：2026-09-16。使用者要求「ID 由 Server 自行生成並保管」，新接入只需 Server URL 與各主機的 Token。驗收依據為 [question.md](question.md)。

## 最終行為

- 面板新增主機時 Server 自動產生 ID 與 Token，ID 留在 Server；接入視窗只顯示一次性 Token，不要求複製 ID。
- 新 Agent 以 Bearer Token 呼叫 `POST /api/ingest`。Server 使用 Token 雜湊索引尋找原主機，平均 O(1) 查找，不逐台掃描。
- 資料庫沿用既有 ID 與 Token 雜湊；重啟從資料庫重建索引，不改 schema、不保存明文 Token。
- Token 更換不改 ID 或歷史，舊 Token 失效；刪除主機後撤銷接入；同名或相同 hostname 不影響各台資料歸屬。
- 舊 `POST /api/ingest/{id}`、`SPM_NODE_ID` 與 `--id` 繼續相容，仍驗證 Token 對應指定主機。
- 兩個安裝器不再詢問 ID；既存設定原樣保留。無 ID 設定不可安裝／降級至不支援此方式的 v0.1.0，拒絕時不替換程式或設定。

## TDD 證據

| 測試 | RED | GREEN |
| --- | --- | --- |
| Server Token 上報生命週期 | 新路徑缺少 CSRF 例外，回 403 | 無 Cookie／CSRF 可用 Token 上報；錯誤 Token 401，多機歷史隔離、重建、輪替與刪除正確 |
| collector 免 ID | 空 ID 被當作無效憑證 | 正確組合含 base path 的新上報路徑、Bearer Token 與間隔 |
| Agent 入口子程序 | 仍要求 SPM_NODE_ID，未上報便退出 | 只給網址與 Token 即收到真實系統樣本 |
| 面板接入資訊 | 仍顯示 SPM_NODE_ID 欄位 | 只顯示 Token，提示設定 Server URL |
| root／免 root 安裝器各 2 例 | ID 仍必填；舊版拒絕訊息未說明版本 | 無 ID 設定可安裝測試新版，舊版清楚拒絕且不破壞已有安裝 |

版本 v0.1.1 在單元測試中是模擬 Release fixture，不代表已公開發布。降級保留斷言在移除初始 ID 阻礙後通過，不另宣稱它曾獨立 RED。

## 本機驗證

- `go test ./... -count=1 -timeout 45s`、`go vet ./...`、`go mod verify` 通過；本機無 PostgreSQL 實例，該整合測試略過。
- Vue 15 項全數通過，Vue LSP 無錯誤，vue-tsc 與 Vite 正式建置通過。
- Python 28 項全數通過：root 安裝器 11、免 root 安裝器 12、Go 測試結果檢查器 5。
- Bash 語法、ShellCheck 0.10.0 與 Go 格式檢查通過。
- 重建 Windows Server／Agent，從環境移除 SPM_NODE_ID 後完成真實兩份樣本、CPU 衍生率、歷史、權限、Token 不洩漏與設定變更；finally 清理子程序，確認服務程序數為 0。
- 主代理自行複查需求、品質、安全、實際行為與歷史相容；未使用子代理，未新增依賴或額外自動註冊功能。

## 遠端與發布

11 筆提交已推送至 `7359ffc8fdc683bb5bbccb41ec1007836f3f02d5`，[本輪 Linux CI](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35028161412) 三個 job 全部成功：

- [Linux race／PostgreSQL／Agent](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35028161412/job/104580072991)：完整 Go race、真實 PostgreSQL 17、15 項 Vue、28 項 Python、型別／建置／靜態檢查、Linux 免 ID 執行檔上報與 ARM64 交叉建置通過。
- [systemd 使用者安裝](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35028161412/job/104580073111)、[Supervisor 使用者安裝](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35028161412/job/104580073192)：UID 1002 一般帳號安裝正式 v0.1.0，驗證舊設定、角色隔離、真實樣本、崩潰重啟、啟停及更新保留。
- 主 job 日誌確認 `TestAgentStartsWithoutNodeID`、`TestUploadWithoutNodeID`、`TestTokenOnlyIngestLifecycle`、`TestPostgresIntegration` 全部 PASS；結果檢查器確認零測試案例略過。
- 新編譯執行檔日誌：`PASS: private access, login, real Agent upload without node ID, CPU rate, history, token redaction, settings`。

兩種非 root job 使用 v0.1.0 及原 ID 設定，屬於舊版相容驗證；本次免 ID Linux 行為由主 job 新編譯產物實測，不冒充免 ID Release 安裝驗收。

目前最新正式版本仍為 v0.1.0，不包含本功能。免 ID 需要先更新 Server 再更新 Agent；只更新新 Agent 而使用舊 Server 會上報失敗。Release smoke 已準備下一版正式發布後的免 ID 安裝驗收，但本輪尚未執行它或發布新版本。

本次共 3 輪工作週期：後端／Agent TDD、面板／安裝器 TDD、整合與交付驗證。ARM64 仍需真機驗證；既有資料與 Token 不需重新註冊。
