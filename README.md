# MABI DPS JP

日本版マビノギモバイルの通信から戦闘イベントを読み取り、自分のDPS、スキル別ダメージ、戦闘履歴を表示するWindowsアプリです。ゲームウィンドウに重ねるミニ表示と、詳細な通常オーバーレイを切り替えられます。

## 利用

Windows 10 / 11 の64ビット環境を対象としています。配布ZIPを展開し、`MabiDPS-JP.exe` を起動してください。基本操作は [使い方.txt](使い方.txt) に記載しています。

リアルタイム計測には [Npcap](https://npcap.com/) を別途導入してください。アダプターを選択して「キャプチャ開始」を押します。サンプル再生でも表示を確認できます。

自キャラクターの自動推定や追加攻撃などの判定は実測に基づく暫定処理です。混雑した場所で自動推定を使う場合は「自分の設定」を確認してください。

## Windows版のビルド

Goの必要バージョンは [go.mod](go.mod) を参照してください。検証に使用した環境は Go 1.27.3 / Windows amd64 です。

リポジトリのルートでPowerShellを開き、次を実行します。

```powershell
.uild-native.ps1 -Go go
```

自動テスト、静的検査、Windows EXEのビルド、サンプル再生のGUI検証を実行します。生成先は `dist-native/` です。フォント・音声・画像・依存Goソースはリポジトリに同梱しています。

ZIPを作成する場合は次を実行します。既存の出力を上書きしないため、新しい出力先を指定できます。

```powershell
.package-native.ps1 -Go go -OutRoot .elease-output
```

通常のWindows版は `native` ビルドタグを使用します。PCAPNGの保存・再生を調査する開発者向けコードは `capturedev` タグを追加したときに有効になります。

```powershell
go test -tags native ./...
go vet -tags native ./...
go build -tags "native capturedev" -o MabiDPS-JP-dev.exe .
```

`web/` とタグなしのGoエントリーポイントは、旧オフラインWeb表示のソースです。Web側を変更する場合は `web/` で `npm ci`、`npm test`、`npm run build` を実行してください。

## 設定と保存データ

設定・履歴・動作ログはEXEの横の `data/` に保存します。旧版からの設定引き継ぎは、初回起動時または「設定 → その他」から実行できます。

個人の設定、履歴、通信記録、ログは公開ソースに含めないでください。ソースの `.gitignore` に除外項目を記載しています。

## ライセンス

アプリケーションコードは [MIT License](LICENSE) です。MOGUGI由来コードの著作権表示、依存ライブラリ、フォント、効果音の条件は [THIRD-PARTY.md](THIRD-PARTY.md) を参照してください。日本語辞書・翻訳データの再配布条件は同文書の確認事項として残っています。

セキュリティレビューの結果は [SECURITY-REVIEW.md](SECURITY-REVIEW.md) に記載しています。
