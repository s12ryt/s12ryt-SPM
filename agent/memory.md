# 操作與決策紀錄

## 2026-09-16：Agent 連線參數

- 依使用者「1，但是 token 也要」確認兩種安裝器與 Agent 啟動均使用 --server／--token；讀取既有入口、安裝器、測試、部署與歷史，寫入 question.md。
- Agent 原有 --server，新增 --token。先以 2 個上報案例及 help 案例重現缺失；參數明確提供時覆蓋環境，預設 help 不帶出環境 Token，空值不回退。
- 兩安裝器的正常參數／help 共 4 例先 RED；完成解析、優先順序、只更新指定欄位與交易失敗還原設定。下載／啟動失敗回復首測即過，列為回歸而非獨立缺陷 RED。
- 設定重寫只處理指定 KEY，不 source 或執行內容，保留其他行與舊 ID；root 採 EnvironmentFile 跳脫，免 root 採純文字原值，維持 0600。文件說明改接不同 Server 時舊 ID 的處理。
- Go 全套、vet、mod verify、34 項 Python（root 14、免 root 15、檢查器 5）、Bash 語法及 ShellCheck 通過。本機 PostgreSQL 沒有實例，完整 race／PostgreSQL 等待遠端本輪 CI。
- 重建 Windows Agent，smoke 清除連線環境變數後以 --server／--token 真實上報，CPU／歷史／權限等 PASS；確認本工作區 Server／Agent 程序數為 0。
- 以需求、品質、安全、實測及歷史五個面向自行複查，沒有呼叫子代理；前端未變更。新增參數驗證文件及使用範例，Release smoke 改為未來發布時使用安裝參數。
- 目前正式 v0.1.1 支援安裝腳本寫入的環境設定，但 Agent 執行檔尚無 --token；未建立下一版標籤或發布新資產。
- 將 15 個檔案依實作與測試分為 8 筆提交，推送至 c00b48813313eaebe2e8fd6b6b1c5d0fd2124e54；本輪 Linux CI 35037897881 三個 job 全部成功。
- 已讀取日誌：完整 Go race／PostgreSQL 17 零案例略過，參數／環境覆蓋／help 測試、34 項 Python、15 項 Vue、靜態檢查及 ARM64 交叉建置通過；新 Linux Agent 僅以 --server／--token 完成真實上報。
- systemd／Supervisor 各以 UID 1002 完成正式 v0.1.0 舊設定安裝相容驗收，沒有冒充新參數的正式 Release 安裝。補齊驗證紀錄，最新公開版本仍為 v0.1.1，下一版發行待確認。
- 文件證據提交推送並確認工作區乾淨後，使用者明確批准發布 v0.1.2；標籤將指向已通過 CI 的 c00b48813313eaebe2e8fd6b6b1c5d0fd2124e54，由既有 Release workflow 重新驗證並實際安裝，不覆寫既有版本。

## 2026-09-16：Agent 免 ID 接入

- 依使用者新要求取代先前三項連線設定；讀取 Server／Agent、兩安裝器、測試、部署及專案歷史，驗收寫入 question.md。
- ID 原本已由 Server 產生；新增 Token 雜湊到 ID 的記憶體索引及 POST /api/ingest，啟動時從既有資料庫重建，不修改 schema 或保存明文 Token。
- 索引在主機新增／Token 更換／刪除成功寫入資料庫後更新，沿用 mutex；舊 POST /api/ingest/{id} 仍驗證指定主機的 Token。
- Server API、collector 空 ID、Agent 入口共 3 例先 RED，再完成實作與 Go 目標回歸；覆蓋同名主機隔離、歷史、重新載入、輪替與撤銷。
- 面板 1 例與兩安裝器 4 例先因 ID 必填／舊版提示缺失而 RED；移除欄位與詢問，僅保留舊環境設定相容。使用有效設定檔阻擋無 ID 降級至 v0.1.0，不 source 或印出秘密。
- 15 項 Vue、28 項 Python、Go 全套／vet／mod verify、型別與建置、Bash 語法及 ShellCheck 通過。本機 PostgreSQL 未提供實例，完整 race／PostgreSQL 待本輪遠端 CI。
- 修改 smoke 不傳或繼承 SPM_NODE_ID，重建 Windows 雙端後真實上報 PASS，已確認本工作區服務程序數為 0。Release smoke 改為未來發布時驗證無 ID 安裝；既有非 root CI 保留正式 v0.1.0 的舊設定相容驗證。
- 以需求、程式品質、安全、實際驗證及歷史五個面向自行複查，沒有呼叫子代理。尚未發布新版本或覆寫 v0.1.0。
- 將 22 個檔案依實作與測試分為 11 筆提交，正常推送至 7359ffc8fdc683bb5bbccb41ec1007836f3f02d5；Linux CI 35028161412 三個 job 全部成功。
- 已讀取三個 job 日誌：完整 Go race／PostgreSQL 17 零案例略過，新增免 ID 測試、15 項 Vue、28 項 Python、Linux 真實免 ID 上報及 ARM64 交叉建置通過；兩種非 root 管理器以正式 v0.1.0 驗證舊設定相容，沒有冒充新版 Release 驗收。
- 最新正式 Release 仍為 v0.1.0；程式與 Linux 產物已可審查，發布下一版需另確認。保存本輪驗證證據，收尾僅更新文件。
- 使用者在完整 CI 成功、文件提交及工作區確認後，明確選擇發布 v0.1.1；將對已驗證的 7359ffc 程式建立版本標籤，再由 Release workflow 全套驗證、發布及正式安裝實測，不覆寫 v0.1.0。
- 推送 v0.1.1 標籤指向 7359ffc8fdc683bb5bbccb41ec1007836f3f02d5；Release workflow 35028677753 全部成功，正式發布四個 Linux 執行檔與 SHA256SUMS。
- 已讀取 publish job 104582452583 日誌，四份校驗均 OK；從公開 Release 安裝的 Agent 只給網址與 Token，真實樣本、systemd、設定權限與更新保留驗收 PASS，測試服務於 finally 清理。
- 透過 gh 確認 v0.1.1 為 latest、非草稿及非預發布；在預核暫存目錄撰寫發行說明後，以 notes-file 更新正式 Release 文字，未修改已發布資產或 v0.1.0。
- README／部署說明改為正式 v0.1.1，保留先更新 Server 再更新 Agent 的順序；更新驗證與任務紀錄。兩種免 root 真實 CI 仍明確標示舊版相容範圍，沒有宣稱新版免 ID 非 root 實測。

## 2026-09-16：Agent 接入設定說明

- 查閱 Agent 入口、免 root 安裝器、README 與前端憑證提示，確認以 `SPM_SERVER` 指定 Server，搭配面板新增主機取得的 `SPM_NODE_ID`／`SPM_TOKEN` 上報；首次安裝會詢問三個值。
- 免 root 安裝將設定保存在 `~/.local/share/spm/agent/config.env`，修改後透過 `spm-user agent restart` 載入；重跑安裝保留既有設定，不重新詢問。此次僅說明既有行為，未變更程式或啟動服務。

## 2026-09-16：具體程式缺陷檢查

- 沿用既有驗收、技術棧及提交／推送授權；讀取實作與測試，全程沒有使用子代理。
- SQLite 特殊目錄與 Linux `?` 檔名先出現有效 RED；查閱 modernc SQLite 官方 file URI／pragma 文件後，以 URL 編碼修復。Windows／Linux store 回歸及 Linux store race 通過。
- Vue 延遲 GET 在主機刪除後仍套用舊資料或錯誤，兩例先 RED；refresh 版本阻擋過時結果後 14 項測試通過。
- 兩安裝器在 binary 替換後注入服務檔寫入 exit 73，確認有效 RED；EXIT trap 回復後原檔及退出碼驗證通過。首次安裝失敗重試兩例首跑即過，僅列回歸。
- 全 24 項 Python 通過；Go 全套、vet、模組驗證、Bash 語法、Vue 型別與建置通過。gopls 不可用，不宣稱 Go LSP 成功。
- Windows binary 重建後真實 Agent smoke 通過，測試 finally 清理子程序；本機安裝器測試僅使用暫存 fixture。
- 對照需求、歷史與改動，自行複查品質、安全及驗證；GitHub 沒有既有 issue／PR。新增 `audit-2026-09-16.md`，遠端 CI 待推送後記錄，未發布或覆寫 Release。
- 六筆提交推送至 `013d7db`；首輪 CI 35019467699 的 systemd／Supervisor 驗收成功，主工作因 ShellCheck SC2154／SC2317 而停止，未執行完整 race／PostgreSQL。
- 將 EXIT trap 抽成 cleanup 區域變數函式；查閱 ShellCheck 上游 #2542，僅對已被退出行為測試覆蓋的回呼註明 SC2317 例外。Debian ShellCheck 0.10.0 解壓在系統暫存，不安裝系統套件；靜態檢查及 24 項 Python 回歸通過，待重跑遠端 CI。
- 推送 `bc0af10a1e829067c701f36db81445e3f4b42728` 後，第二輪 CI 35020366845 三個 job 全部成功；已讀取日誌，完整 Go race／PostgreSQL 17 必要案例全部 PASS，零測試案例略過。
- 同次 Vue 14 項、Python 24 項、型別／靜態檢查、Linux 新執行檔真實上報及 ARM64 交叉建置通過；兩種管理器各以 UID 1002 完成正式 Release 安裝、秘密原值、崩潰重啟、啟停與更新保留。
- 更新本輪稽核證據、任務及驗證索引；收尾僅文件。確認最新正式 Release 仍為 v0.1.0，其 Go／Vue 執行檔未含本輪修復，未新增或覆寫 Release。

## 2026-09-15：免 root VPS 安裝

- 重讀安裝器、測試、CI、部署與三份專案紀錄，原 7 項安裝器測試通過。
- question 確認 systemd --user 優先、獨立 venv Supervisor 備援，依賴及 linger 限制寫入 question.md。
- 查閱 Supervisor 官方文件及 Context7，採私人 UNIX socket、固定 4.3.0、程序群組停止與有限日誌輪替。
- 免 root 行為測試先出現 5 個有效 RED；另修正測試 fixture 未建立空操作紀錄的問題，該 fixture 錯誤不算產品缺陷。
- 完成使用者安裝與管理工具，追加有效 RED 發現背景程序繼承鎖描述符，關閉描述符後 8 個目標測試及全 20 個 Python 測試通過。
- 本機只執行隔離 HOME／模擬主機命令的有限測試，沒有安裝或啟動正式本機服務。
- CI 新增兩個隔離非 root 帳號驗收；只有 runner 準備帳號／linger 使用管理權限，安裝器及 SPM 全程一般 UID。
- 首輪 run 34957094027 因一般帳號無法讀取 runner checkout 而未進入安裝器；複製兩個必要檔案至測試帳號家目錄，不改 runner 私人目錄權限。
- 修復提交 3670974e18445f97ceb6358aad5c60a8abe52fdb；第二輪 run 34957308410 三個 job 全部 success。
- systemd／Supervisor job 均以 UID 1002 真實下載正式 v0.1.0，完成角色隔離、秘密原值、兩筆真實樣本、崩潰重啟、啟停、更新保留與檔案權限驗證；finally 與 workflow 均清理測試服務。
- 讀取 job 日誌確認兩種管理器 PASS；同次完整 Go race／PostgreSQL、20 項 Python、12 項 Vue、靜態檢查、binary smoke 及 ARM64 交叉建置通過。
- 公開 raw 的 install-user.sh 可未登入讀取；新增 user-install-validation.md，收尾為文件更新，不重新發布或覆寫 v0.1.0。

## 2026-09-15：公開與 Release 安裝

- 使用者明確授權公開儲存庫，已透過 gh repo edit 切換並確認 PUBLIC。
- question 確認不加參數只裝 Server、`agent` 只裝 Agent，版本標籤觸發，首版 v0.1.0；已寫入驗收。
- 新安裝器先 stub 後 7 組行為測試出現 5 個有效 RED，實作後 GREEN；另以 RED 發現啟動失敗未還原 service 檔，完成修復。
- WSL 隔離安裝測試只模擬網路及主機服務命令，並寫入暫存根目錄，沒有修改本機 /etc、/opt 或啟動長駐服務。
- 下載 Debian shellcheck 套件到系統暫存並解壓執行，沒有系統安裝或提權。ShellCheck、bash -n、12 項 Python 測試及 actionlint 通過。
- Release workflow 重用完整 CI，發布後直接下載正式 Release 驗證 systemd、角色隔離、特殊字元密碼及真實 Agent；遠端結果待記錄。
- 提交 7c20ccb0a98da5fb81536d936993dcf10e620aeb 已推 main 與 v0.1.0 標籤；Release run 34953718316、main CI run 34953715108 均 success。
- v0.1.0 正式發布四個 Linux 執行檔與 SHA256SUMS；發行頁非草稿、非預發布。
- publish job 真實從公開 Release 下載並完成 systemd 安裝，驗證最新版本解析、角色隔離、特殊字元密碼、Agent 兩筆真實樣本及更新保留資料；finally 停用／停止測試服務。
- 公開 raw 安裝腳本可未登入讀取。新增 release-validation.md，修正 systemd 歷史限制，收尾僅文件提交不更動已發布產物。

## 2026-09-15：GitHub CI

- 使用者要求上傳 GitHub 執行真實 Linux CI，授權建立版本庫、提交與推送。
- 確認 gh 已登入 s12ryt，無既有 SPM 儲存庫；建立私有 https://github.com/s12ryt/s12ryt-SPM。
- 初始化 main 分支，沿用既有 Git 使用者設定；不修改 Git config。
- 新增 Linux CI：Ubuntu 24.04、PostgreSQL 17 service、Go 1.26.3、Node 24、前端測試／型別／建置、完整 race、Agent smoke、ARM64 交叉建置與證據 artifact。
- CI 強制 PostgreSQL 和三項原 WSL 受阻 HTTP 測試為 PASS，任何測試案例 SKIP 均失敗。actionlint 本地通過。
- 首輪遠端完整 race／PostgreSQL 已通過，但 JSON 檢查器誤判無單元測試的 Agent 入口套件 skip，阻止後續步驟。以 5 項 Python 測試先重現 2 個檢查器缺陷，再修正至全綠；以首輪真實 artifact 複驗通過並推送重跑。
- 第二輪 run 34950376689／提交 a17530c393fbb46f6b01fe5d4ee5dba2a0b6437f 最終 success：完整 race、PostgreSQL 17、12 項前端測試、5 項 Python 測試、Linux 真實 Agent smoke、ARM64 交叉建置與 artifact 均通過。
- 下載第二輪測試 JSON 至系統暫存目錄再次通過檢查器，確認必要案例全部 PASS 且零案例略過；測試與執行檔 artifact 保留 14 天。
- 更新 CI 證據與歷史限制；收尾為僅文件的 `[skip ci]` 提交，不重寫歷史或變更已驗證程式。

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
