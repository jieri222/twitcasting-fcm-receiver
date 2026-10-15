# TwitCasting FCM Receiver

> 專案說明：自架式 TwitCasting Web Push 接收器，解密通知並輸出至標準輸出或自訂 Webhook。
>
> 作者／維護者：[@jieri222](https://github.com/jieri222)
>
> 專案網址：<https://github.com/jieri222/twitcasting-fcm-receiver>

接收 TwitCasting Web Push 通知，解密後輸出到標準輸出；也可以選擇轉送到自訂 Webhook。

本專案為非官方工具，與 TwitCasting／Moi、Google 或 Mozilla 無隸屬或背書關係。

## 功能

- 連線至 Mozilla Autopush WebSocket，接收 Web Push 通知。
- 使用本機保存的金鑰解密通知。
- 將解密內容輸出到標準輸出，或選擇轉送至 `WEBHOOK_URL`。
- 使用 Firebase Installations 與 FCM 註冊推播 token，並可回報給 TwitCasting。
- 使用 `--list-subs` 列出目前帳號的訂閱頻道。

### 訂閱管理尚未完成

訂閱和取消訂閱的端點已確認可用，但尚未整合成命令列流程。`--list-subs` 目前只用來列出訂閱；目前不能透過命令列新增或移除訂閱。

## 需求

- Go 1.26.2 或更新版本。
- 要使用 FCM 註冊功能，需設定 Firebase 專案參數。
- 要列出訂閱或向 TwitCasting 回報 FCM token，需提供有效的登入 Cookie。

## 設定

複製 `.env.example` 為 `.env`，填入需要的值：

| 變數 | 必要性 | 說明 |
| --- | --- | --- |
| `FCM_API_KEY` | FCM 註冊必要 | Firebase 專案 API key。 |
| `FCM_PROJECT_ID` | FCM 註冊必要 | Firebase 專案 ID。範本預設為 `twitcasting-web-push`。 |
| `FCM_APP_ID` | FCM 註冊必要 | Firebase Web App ID。 |
| `TC_COOKIE` | 選用 | TwitCasting 登入 Cookie；需要回報 FCM token 或列出訂閱時使用。請勿分享或提交。 |
| `TC_DEVICE_ID` | 選用 | 覆寫回報 FCM token 時使用的 device ID；未設定時從 `TC_COOKIE` 的 `web_devid` 讀取。訂閱管理會直接使用 Cookie 中的 `web_devid`。 |
| `PUSH_URL` | 選用 | Autopush WebSocket URL；預設為 `wss://push.services.mozilla.com/`。 |
| `STATE_FILE` | 選用 | 狀態檔路徑；預設為目前目錄的 `state.json`。 |
| `VAPID_KEY` | 選用 | 傳給 Push WebSocket 註冊訊息的 VAPID 公鑰。 |
| `FCM_VAPID_KEY` | 選用 | 傳給 Firebase FCM 註冊請求的 VAPID 公鑰。 |
| `WEBHOOK_URL` | 選用 | 通知解密後會以 `application/octet-stream` POST 到此網址。 |

FCM 註冊參數未設定完整時，程式仍可嘗試接收 Web Push，但會略過 FCM 註冊。

### 使用 Cookie 的影響與風險

`TC_COOKIE` 是登入憑證，不是一般設定值。程式會把它附加在 TwitCasting 網站或前端 API 的請求上；持有有效 Cookie 的程式或人員，可能在 Cookie 權限範圍內代表你使用帳號。Cookie 的實際權限、有效期限，以及 TwitCasting 是否會因這類非官方自動化請求要求重新登入或採取其他安全措施，都無法由本專案確認。

請只使用自己的帳號 Cookie，不要使用他人帳號或把 Cookie 貼到 issue、聊天訊息、截圖或日誌。建議在專用環境執行，限制 `.env` 和執行環境的存取權；若懷疑 Cookie 外洩，請登出或撤銷相關登入工作階段。即使不設定 `TC_COOKIE`，程式仍可能使用已保存的 `state.json` 和 FCM 設定接收推播。

程式預設從 `TC_COOKIE` 內沿用網站提供的 `web_devid`。`TC_DEVICE_ID` 只會覆寫回報 FCM token 時送出的 device ID；訂閱管理仍直接使用 Cookie 中的 `web_devid`。目前沒有讓程式自行產生 ID：網站端如何驗證或關聯 `web_devid` 沒有公開說明，自行產生可能被視為另一台裝置、建立額外註冊，或被端點拒絕。建議先沿用 Cookie 中的值，也不要設定與 Cookie 不同的 `TC_DEVICE_ID`，除非已確認服務端接受這種組合。

## 執行

啟動通知接收器：

```sh
go run .
```

列出登入帳號的訂閱頻道：

```sh
go run . --list-subs
```

顯示除錯日誌：

```sh
go run . --verbose
```

建立執行檔：

```sh
go build .
```

## 輸出與 Webhook

每則成功解密的通知會輸出到標準輸出。若設定 `WEBHOOK_URL`，程式也會將通知明文以 HTTP POST 轉送；Webhook 呼叫逾時為 15 秒。請只設定自己信任且使用 TLS 的接收端點，並確認其存取控制與資料保存方式。

## 敏感資料

- `.env` 內的 `TC_COOKIE` 可用來代表登入帳號，請勿提交到版本控制或貼到公開 issue。
- `state.json` 會保存 Push 私鑰、FCM Installation refresh token 與推播 token。請妥善保護並勿公開；若外洩，請撤銷相關憑證或重建狀態。
- Webhook 接收的是解密後的通知內容，請確認接收端有適當的存取控制。

## 服務相容性

本程式使用 TwitCasting 網站前端的推播註冊端點。該端點可能不屬於公開 API，且可能隨服務更新而變更。使用前請自行確認符合 TwitCasting 的現行服務條款；本工具不保證端點持續可用。

## 授權

本專案目前附有 GNU General Public License v3.0（GPL-3.0），詳見 [`LICENSE`](LICENSE)。

專案版權所有者：

```text
Copyright (C) 2026 jieri222
```

## 聯絡方式

- 問題回報：<https://github.com/jieri222/twitcasting-fcm-receiver/issues>
- 聯絡信箱：
