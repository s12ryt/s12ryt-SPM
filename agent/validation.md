# 驗證紀錄（2026-09-15）

以下保留首次交付紀錄。使用者要求仔細檢查後的最新結果見 [深入稽核紀錄](audit-2026-09-15.md)：Go 全套／vet 通過、Vue 增至 12 項測試、選定範圍 race 通過，並重新建置及驗證 Windows／Linux 執行檔。完整 HTTP race 與 PostgreSQL 實例限制仍保留。

## 範圍與輪次

依 `question.md` 驗收。共 7 個工作週期：資料模型、儲存／搬移、HTTP 權限與 API、通知／採集、Vue、整合，以及回歸／交付收尾。每個實作週期內可有多次 RED → GREEN；此處不將指令重試或環境安裝失敗計為產品 RED。

## TDD 證據

| 行為 | RED 觀察 | GREEN 與對應測試 |
| --- | --- | --- |
| 樣本、速率、規則、狀態轉換 | stub 無法計算速率、拒絕非法資料或產生告警 | `internal/model/model_test.go` 全通過 |
| 儲存、分桶、完整搬移 | stub 未保存狀態、歷史及目標資料 | `store_test.go`：往返、20→5 桶、目標交易、重試與 occupied 拒絕 |
| 重啟切換與環境優先 | 缺少排程／載入搬移行為 | `runtime_test.go`：環境優先、重啟切換、失敗保留來源 |
| 通知佇列及保留期限 | 沒有待送事件、退避及清理 | `maintenance_test.go` 全通過 |
| API、權限、上報、告警 | 未實作 handler 返回 404 | `server_test.go`：Session、CSRF、Token、公開／私人、歷史、間隔、離線與恢復去重 |
| Webhook／Telegram | stub 沒有送出請求 | `notify_test.go`：Webhook JSON／503、Telegram 格式與停用管道 |
| 真實採集、上報及退出 | stub 樣本驗證失敗、無 HTTP 上報／不進入循環 | `collector_test.go`、`run_test.go`：有效樣本、下發間隔、錯誤、不合法 URL、取消 |
| 靜態面板／favicon | 入口／favicon 返回 404 | `web/embed_test.go`：首頁掛載點、缺失資源 404、favicon 200 |
| Vue 權限及總覽 | 初版 3 個行為測試失敗 | `web/src/App.test.ts`：登入、總覽／搜尋、管理控制及錯誤 |
| 主機排序／覆寫規則 | map 順序跳動；設定全域 15 秒後覆寫仍為 3 秒 | `regression_test.go` 排序；Vue 覆寫測試通過 |
| 同名告警／磁碟歷史 | 兩台同名主機只算 1 筆；缺少磁碟圖 | Vue 2 項回歸通過 |
| 慢請求隔離 | 5 個 mutation handler 讀取卡住時 GET session 超時 | `TestSlowBodiesDoNotBlockMonitoring` 5 個端點通過 |

## 首次交付驗證

| 檢查 | 結果 |
| --- | --- |
| `go test ./... -count=1 -timeout=45s` | 通過；cmd 入口由 binary smoke 驗證，PG 選用測試 skip |
| `go vet ./...` | 通過 |
| `npm --prefix web test` | 7 個測試通過 |
| `npm --prefix web run build` | Vue 型別檢查與 Vite build 通過 |
| Go Windows amd64 server／agent build | 通過 |
| Go Linux amd64／arm64、CGO=0 server／agent build | 共四個產物通過；ARM64 未實機執行 |
| Windows binary smoke | 通過：私人權限、登入、真實 Agent、CPU、歷史、Token 不外洩與設定 |
| WSL Linux amd64 binary smoke | 通過，同上；本機 port 18081 |
| WSL `go test -race ./internal/model ./internal/store -count=1 -timeout=30s` | 通過；Go 1.26.3 |
| 全套 `go test -race ./internal/...` | 未通過：collector／通知 HTTP 測試受 WSL loopback connection refused 阻礙 |
| `TestPostgresIntegration` | 略過：未提供 TEST_POSTGRES_URL，沒有執行 PostgreSQL 實例驗證 |
| gopls 診斷 | 未執行成功：環境未安裝，已使用編譯／vet／測試檢查 |
| Agent 依賴樹 | 無 SQLite、pgx、store、Vue |

前端產物：JS 94.52 kB／gzip 35.90 kB；CSS 11.62 kB／gzip 3.49 kB。這是本次建置尺寸，不是執行期記憶體數據。

## 瀏覽器實測

- 使用隔離本機資料庫；管理員密碼與 Agent Token 僅限測試暫存環境。
- 公開訪問、管理員登入、全域採樣間隔 3→5 秒儲存、CPU／磁碟等歷史圖。
- 新增測試主機、一次 Token 顯示、確認刪除，再回到只有真實採集主機。
- 最終版 1440px 桌面與 375px 手機寬度無水平溢出；讀取實際截圖核對版面。
- 詳情有 5 個圖表（磁碟、CPU、RAM、接收、傳送）；favicon HTTP 200。
- 服務存活時最終頁面外部資源請求為空；console error／warning 均為 0。TTL 結束後面板仍曾嘗試輪詢而累積連線錯誤，隨後已關閉測試頁面。
- 測試程序有最長 TTL 與 finally 清理；最後確認 Windows／WSL 無本次服務程序。

## 尚未驗證與使用邊界

- PostgreSQL 真實建表、讀寫與雙向搬移需執行 opt-in 整合測試，未以 SQLite 測試冒充 PostgreSQL 通過。
- WSL 獨立 httptest 小程式在 Go 1.26.0／1.26.3 同樣 connection refused；固定 port 的 Linux 真實服務正常。沒有發現 data race 報告，但這不等於全套 race 通過。
- 未使用真實 Telegram Bot 或外部 Webhook 收件端；目前有 HTTP 請求格式、失敗與佇列模擬測試。
- ARM64 為交叉建置；systemd、TLS 反代範本尚未在正式 VPS 安裝；未做長期多機壓力與容量測試。
- 單主程式設計、通知可能重送、Agent 斷線不補傳；告警摘要依最近 100 個事件計算。實際部署與備份方式見 README。
