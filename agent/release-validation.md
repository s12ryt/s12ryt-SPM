# 公開儲存庫與 Release 驗證（2026-09-15）

## 已發布版本

- 公開儲存庫：https://github.com/s12ryt/s12ryt-SPM
- 正式版本：https://github.com/s12ryt/s12ryt-SPM/releases/tag/v0.1.0
- 標籤提交：`7c20ccb0a98da5fb81536d936993dcf10e620aeb`
- Release workflow：https://github.com/s12ryt/s12ryt-SPM/actions/runs/34953718316 — **success**。
- 同提交 main CI：https://github.com/s12ryt/s12ryt-SPM/actions/runs/34953715108 — **success**。
- 發布時間：2026-09-15 09:41:24 UTC；非草稿、非預發布版本。

| Release 資產 | bytes | 驗證 |
| --- | ---: | --- |
| `spm-server-linux-amd64` | 15,216,802 | 真實 systemd 安裝、登入與更新保留 |
| `spm-agent-linux-amd64` | 6,709,410 | 真實 systemd 安裝、採集與上報 |
| `spm-server-linux-arm64` | 14,483,618 | 交叉建置與 SHA-256 |
| `spm-agent-linux-arm64` | 6,226,082 | 交叉建置與 SHA-256 |
| `SHA256SUMS` | 354 | 四個檔案校驗成功 |

安裝器的下載位址只有 `github.com/s12ryt/s12ryt-SPM/releases/…`；未從原始碼、Actions artifact 或其他儲存庫安裝。公開 raw `install.sh` 已由未帶登入憑證的讀取工具確認可用。

## TDD 與靜態驗證

第一輪使用有效 Bash stub，7 組測試有 5 組因未實作安裝、角色選擇、設定保存／引用、服務失敗回復而 RED；完成實作後全部 GREEN。下載失敗及非法輸入測試在 stub 階段即通過，不把這兩組初次結果算成缺陷修正。

第二輪增加「更新服務失敗後還原原 service 檔」斷言，重現只有執行檔被還原的問題；修復後 GREEN。成功更新保留 env／資料，服務失敗同時回復既有 binary／unit。

- `scripts/test_install.py`：7 組通過，內含角色／架構、固定版本、下載失敗、校驗錯誤、平台／URL／密碼／Token 邊界、更新保留、秘密不輸出與回復。
- `scripts/test_verify_go_tests.py`：既有 5 項通過；Python 總計 12 項。
- `bash -n install.sh`、ShellCheck 0.10.0、actionlint 均通過。
- Ubuntu workflow：Vue 12 項測試、型別與 build、Go 格式／vet／依賴校驗、含 PostgreSQL 17 的完整 race、真實 Linux binary smoke 全部通過，沒有排除任何 Go 測試案例。

## 真實 Release 安裝驗收

`publish` job（104331106849）在新 Ubuntu 24.04.5 runner 執行 `timeout 240s sudo python3 scripts/release_smoke.py "$TAG"`。

1. 預設不加角色或版本參數，解析最新正式版本 v0.1.0，僅安裝 Server。
2. 使用含空白、`$`、雙引號與反斜線的測試密碼，systemd EnvironmentFile 載入後成功登入。
3. 建立主機，使用 `agent --version v0.1.0` 只安裝 Agent，Server 執行檔與設定不變。
4. 真實 Agent 上傳至少兩筆系統樣本，CPU 衍生率可用且主機 online。
5. 重跑 Server 安裝，保留管理員密碼、主機資料與 Agent 設定；可重新登入。
6. 檢查兩份設定權限 0600，兩個服務 is-active。
7. finally 停用並停止兩個測試服務，runner 正常清理。

實際日誌：`PASS: public Release assets, role isolation, SHA256, systemd, real Agent samples, update preservation`。

## 範圍與限制

- 共 3 輪工作週期：安裝器 TDD、失敗回復回歸、遠端 Release 發布與真實安裝驗收。首個 Release workflow 一次成功。
- ARM64 僅交叉建置；未宣稱 ARM64 真機或所有 Linux 發行版均實測。安裝器需要 systemd，OpenRC 不在範圍內。
- 啟動失敗回復執行檔／service 檔，不回復資料庫 schema 或內容；正式升級前仍應備份資料。
- Actions upload-artifact@v4／download-artifact@v5 有 Node 20 棄用註記，runner 已強制使用 Node 24 並成功；後續可更新 Action 主版本。
- 對外 HTTPS 與防火牆由 VPS 管理者設定；本次未配置使用者的 VPS 或外部通知。
- 收尾只提交文件 `[skip ci]`，不移動版本標籤、不覆寫 Release 資產；已驗證程式及 workflow 保持不變。
