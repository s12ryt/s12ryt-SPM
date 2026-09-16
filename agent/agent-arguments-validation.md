# Agent 參數配置驗證

日期：2026-09-16。使用者確認一鍵安裝與 Agent 啟動都要以 `--server URL --token TOKEN` 指定連線。需求來源與更新契約見 [question.md](question.md)。

## 行為

- Agent 明確參數覆蓋 `SPM_SERVER`／`SPM_TOKEN`，未給參數仍沿用環境變數；明確空值拒絕啟動。
- root 與免 root 安裝均接受兩參數及 `--key=value`；首次缺值沿用環境變數或詢問。
- 既有安裝未給參數時設定不變；只替換明確指定的欄位，保留其他設定、註解及舊版 ID。
- 設定保持 0600；下載失敗不更動，替換或啟動失敗嘗試還原原設定及執行檔。
- Token 不出現在程式 help、安裝日誌或驗證錯誤中；命令列本身仍可能被 shell 歷史及程序列表看見，環境變數方式保留。
- 沿用免 ID 與舊 ID 相容；Server API、資料庫及前端無變更。

## TDD

| 範圍 | 有效 RED | GREEN |
| --- | --- | --- |
| Agent 參數 | 只用參數與覆蓋環境兩例因未知 --token 而未上報 | 真實子程序向預期 URL 送出指定 Token |
| Agent help | 沒有 Token 參數說明 | help 含參數說明且不帶出環境秘密 |
| 兩種安裝器 | 正常參數與 help 四例因未支援參數而失敗 | 新裝、指定欄位更新、特殊字元、權限、無參數保留均通過 |

空值／缺值與下載／服務失敗回復等追加檢查首次即通過，僅列回歸，不宣稱獨立缺陷 RED。安裝器測試使用隔離暫存目錄與模擬下載／服務，不改動本機正式服務。

## 本機驗證

- Go 全套、go vet、模組驗證通過；PostgreSQL 未提供本機實例，完整 race 與真實 PostgreSQL 已由下列本輪 Linux CI 補足。
- 34 項 Python 通過：root 安裝 14、免 root 安裝 15、Go 結果檢查器 5。
- Bash 語法與 ShellCheck 0.10.0 通過；Go 格式及 git diff 檢查通過。
- 重建 Windows Agent，清除繼承的連線設定後，只用 --server／--token 向實際 Server 上報，兩份樣本、CPU、歷史、權限與 Token 遮蔽通過；確認無測試服務殘留。
- 前端未修改；主代理自行複查需求、品質、安全、驗證及歷史，未使用子代理。

## 遠端與發行

- [Linux CI 35037897881](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35037897881) 三個 job 全部成功。
- 驗證程式提交：`c00b48813313eaebe2e8fd6b6b1c5d0fd2124e54`。
- [完整回歸與整合](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35037897881/job/104611018706)：Go 全套 race、真實 PostgreSQL 17、15 項 Vue、34 項 Python、型別／靜態檢查、Linux 執行檔整合與 ARM64 交叉建置全部通過。
- [systemd 一般帳號](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35037897881/job/104611018447) 與 [Supervisor 一般帳號](https://github.com/s12ryt/s12ryt-SPM/actions/runs/35037897881/job/104611018606)：各以 UID 1002 驗證正式 Release 安裝、角色隔離、上報、崩潰重啟、啟停及更新保留。

已讀取日誌確認新增參數、覆蓋順序、help 與錯誤測試成功；檢查器確認 `zero skipped test cases`。主 job 使用新建置的 Agent，真實整合輸出：

```text
PASS: private access, login, real Agent upload using --server/--token without node ID, CPU rate, history, token redaction, settings
```

兩種非 root job 仍使用 v0.1.0 驗證舊設定相容，不能當作新版參數安裝實機證據。

正式 v0.1.1 可由新版安裝腳本將參數寫入環境設定使用；直接執行 Agent 的 --token 仍需要本輪新版執行檔。尚未發布下一版；Release smoke 已改為未來發布時從公開 Release 驗證參數安裝，不宣稱已執行。

ARM64 尚未真機驗證。更新回復仍屬盡力還原，不包含資料庫、持續磁碟寫入失敗或斷電。共 3 輪工作週期：Agent 參數 TDD、安裝器 TDD、整合與交付驗證。
