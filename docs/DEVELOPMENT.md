# 開発ガイド

## 構成

- `app/`：Windowsアプリ、集計処理、テスト
- `app/assets/`、`app/fonts/`、`app/sounds/`：同梱素材
- `app/config/`：表示名・地域などの設定
- `app/vendor/`：依存ライブラリ。ネットワーク取得をせずにビルドできます
- `docs/`：利用・開発文書
- `.github/workflows/`：CI

## ビルド

Windows 10/11 x64とgo.mod記載のGoを使用します。PowerShellをルートで開いて実行してください。

```powershell
.\build-native.ps1 -Go go
.\package-native.ps1 -Go go -OutRoot .\release-output
```

テスト、静的検査、EXE作成、サンプル再生によるGUI検証を実行します。通常のビルドタグはnativeです。

```powershell
cd app
go test -tags native ./...
go vet -tags native ./...
```

PCAPNG調査機能は開発者向けcapturedevタグでのみ有効になります。

## データの流れ

キャプチャ → TCPストリーム再構築 → JP9フレーム解析 → ダメージ・関連通知の照合 → 戦闘集計 → Windows UI。
表示名の変更・同名行の統合は表示層で行い、元のイベントIDとダメージを保持します。対応を確認できない効果は未分類に残します。

## 辞書と個人データ

識別子翻訳.csvは必須の同梱ファイルです。辞書欠落時はビルドを停止します。再配布条件はTHIRD-PARTY.mdを参照してください。
設定・履歴・通信記録・ログをコミットしないでください。ローカルキャプチャに依存するテストは環境変数を指定した場合のみ実行し、未指定ではスキップします。

## 削除した旧構成

旧ブラウザー版とmodernpreviewタグ向けWebView試作は現行Windows版では使用しないため削除しました。Node.jsは不要です。機能に対応する回帰テストは保持しています。
