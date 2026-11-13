# 自動更新

起動時に公開GitHub Releaseを1日1回確認します。ダウンロード・適用にはユーザーの操作と確認が必要です。

- タグ：v1.2.3、配布ZIP：MABI-DPS-JP-v1.2.3-Windows.zip
- VERSIONとrelease-manifest.jsonを一致させてください
- HTTPS、取得元制限、SHA-256照合、展開パス・容量制限を行います
- 既存data・CSV・config・soundsは保持します。既存辞書は自動で上書きしません
- バックアップはEXE横の.updates/pending-*に残ります

独立した鍵による更新署名は未導入です。GitHubアカウントやReleaseが侵害された場合を防ぐ仕組みではありません。電源断などの中断からの自動復旧は保証しません。
