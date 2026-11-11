# ビルド

Windows上でGo 1.25以上を用意し、jpフォルダーで次を実行します（この版はGo 1.27.3で確認）。

```powershell
.\build-native.ps1 -Go go -OutDir .\dist
```

vendorを同梱しています。標準ビルドはWebView2を使用しません。既存のサンプルGUIテストも実行します。`-classic-ui`は従来の画面へ戻す実行オプションです。

modern_windows.go、modern_overlay_windows.go、modern-uiはWebView2の未完成試作です。modernpreviewビルドタグのため標準ビルドには入りません。白画面が未解決であり完成機能として扱いません。vendorのgo-webview2ではコントローラー表示を明示する試作変更を含みますが、今回のEXEには使用されません。通常の検証は `go test -tags native ./...` と `go vet -tags native ./...` です。

実PCAPNGの追加検証は `MABI_LOCAL_RESEARCH` 環境変数で既存outputsフォルダーを明示した場合だけ実行します。生通信はソースZIPに含めません。出力するidentity-research.jsonは職業文字列の配置・匿名ID一致件数・暫定集計の報告です。

元プロジェクトのMITライセンスとvendorのライセンスを維持してください。Mobinogi_Meterは仕様比較に使用し、ライセンスが明示されていないコードの転記は行っていません。Scouter Liteのコード・ロゴ・画像は利用していません。
