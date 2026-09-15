# 操作與決策紀錄

## 2026-09-15：GitHub CI

- 使用者要求上傳 GitHub 執行真實 Linux CI，授權建立版本庫、提交與推送。
- 確認 gh 已登入 s12ryt，無既有 SPM 儲存庫；建立私有 https://github.com/s12ryt/s12ryt-SPM。
- 初始化 main 分支，沿用既有 Git 使用者設定；不修改 Git config。
- 新增 Linux CI：Ubuntu 24.04、PostgreSQL 17 service、Go 1.26.3、Node 24、前端測試／型別／建置、完整 race、Agent smoke、ARM64 交叉建置與證據 artifact。
- CI 強制 PostgreSQL 和三項原 WSL 受阻 HTTP 測試為 PASS，任何 SKIP 均失敗。actionlint 本地通過，遠端結果待記錄。

## 2026-09-15

- 讀取工作區，確認沒有既有檔案或專案規範。
- 嘗試讀取三份專案紀錄，均不存在，因此建立紀錄。
- 讀取 `C:/Users/yoyo2/.config/opencode/skills/tdd-awa/SKILL.md`。
- 檢查可執行工具：Go、Node.js、npm 可用，未發現 Docker 指令。
- 使用 question 工具確認重要技術選擇，結果寫入 `agent/question.md`。
- 全程由主代理執行，不使用子代理。
- 建立 model／store／server／collector 的失敗測試，逐組完成實作與 GREEN。
- 通知採持久化 outbox、有限次數退避；資料庫切換採目標交易與 migration ID 重試。
- Windows 真機採集通過；Agent 僅含採集與 HTTP 上報依賴。
- 建立 Vue 面板測試，先觀察私人登入、總覽與管理控制失敗再實作。
- 修正 vue-tsc 不相容 TypeScript 7，固定 TypeScript 5.9；前端建置成功。
- 回歸測試重現主機排序不穩及單機覆寫未繼承全域規則，完成修正。
- 建立具 900 秒最長存活與 finally 清理的隔離整合測試，資料放系統暫存目錄。
- 查驗 WSL Debian 可用，未擅自提升 Docker 權限；目前沒有可用 PostgreSQL 實例。
- 在 Windows 與 WSL Linux 分別執行真實 binary smoke，驗證登入、私人權限、兩份真實樣本、CPU 衍生率、歷史與公開設定。
- Playwright 實測管理員登入、全域間隔 3→5 秒、主機新增／Token 顯示／刪除及詳情圖。
- 補失敗測試，修正同名主機告警合併及缺少磁碟歷史；前端目前 7 項測試。
- 以 5 個卡住 request body 的端點重現全域 mutex 阻塞；將解碼移至 mutex 外，完整回歸通過。
- 移除 Google Fonts 依賴，改用系統字型；加入本地 SVG favicon 並完成 404→200 測試。
- 格式化前端原始碼，Vue 型別檢查與 Vite build 通過；JavaScript 94.52 kB（gzip 35.90 kB）。
- WSL HTTP race 與獨立 httptest probe 均出現 loopback connection refused，Go 1.26.0／1.26.3 都重現；model／store race 通過，完整 HTTP race 不標成通過。
- 新增 opt-in PostgreSQL 測試，使用隨機 schema 覆蓋雙向搬移／分桶／狀態／佇列；本機無 TEST_POSTGRES_URL，明確 skip。
- gopls 未安裝；以 Go 編譯、完整測試及 go vet 驗證，未宣稱 LSP 診斷成功。
- 建置六個最新版產物：Windows amd64 與 Linux amd64／arm64，各含 server／agent。
- 最終 Playwright：1440px／375px 無水平溢出、5 張圖、favicon 200、零外部資源請求、console 無 error／warning；已讀取實際桌面與手機截圖。
- 最終 300 秒瀏覽器 fixture 到期自動清理；已查驗 Windows／WSL 無本次 server／agent／Python 測試程序殘留，關閉測試瀏覽器。
- 建立 README、Linux systemd／env 範本，更新任務、依賴與驗證紀錄。未建立正式帳密、未部署外部服務、未執行 git commit。

## 2026-09-15：使用者要求仔細檢查後

- 重新讀取驗收、紀錄、核心程式、前端與測試；沿用技術棧及產品範圍，由主代理完成稽核。
- 基線既有 Go 與 7 項前端測試通過後，逐項注入延遲／斷線／權限到期／寫入失敗等情境。
- 新增 model／server／store 的 audit_test.go、cmd/server/main_test.go 及 Vue App.audit.test.ts。告警、前端狀態、URL／環境優先、HTTPS Cookie、失敗 exit code、共用鎖問題均有先 RED 後 GREEN 證據。
- 搬移中途失敗、10 條私人與逾期端點、通知 307 不轉送秘密首次測試即通過，不計為已修缺陷。
- 完整 Go 測試與 vet 通過；Vue 12 項測試、vue-tsc、Vite build 通過。JS 95.31 kB／gzip 36.16 kB。
- 在 WSL Go 1.26.3 執行核心 race，明確排除 TestUpload、TestRunUploadsAndStops、TestWebhookPayloadAndFailure 三項動態本機 HTTP 測試，其餘 collector／model／server／store 通過。沒有宣稱完整 race 通過。
- 六個 Windows amd64／Linux amd64／arm64 執行檔已依稽核修正版重建；Windows 與 Linux amd64 真實 binary smoke 通過。
- 明確重跑 PostgreSQL 選用測試，因未設定 TEST_POSTGRES_URL 而 SKIP。
- 最終瀏覽器確認 Cookie 失效後清除管理表單、外部 API 刪除選中主機後返回總覽、5 張歷史圖與 375px 無溢出；console 0 errors／0 warnings。已關閉瀏覽器頁面。
- 180 秒測試 fixture 改由 Python detached 模式啟動，具有 TTL 與 finally 清理；未對正式服務、使用者資料庫或外部通知做寫入。
- 更新 README、任務、依賴表與原始驗證紀錄，新增 audit-2026-09-15.md 詳列本次 5 輪稽核與驗證限制。
- 最終查詢 Windows 本工作區 server／agent 及 detached fixture PID 27856、WSL 本工作區 binary，均無程序殘留；測試瀏覽器已關閉。
