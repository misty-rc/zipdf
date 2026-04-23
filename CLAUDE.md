# zipdf

ZIPアーカイブ内の画像をファイル名順でPDFに結合するコマンドラインツール。

## ビルド・実行

```bash
# ビルド
go build -o zipdf.exe .

# 実行（archiveディレクトリ内の *.zip を処理）
./zipdf -i ./archive

# カレントディレクトリを処理（デフォルト）
./zipdf
```

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | フラグ解析・ディレクトリスキャン・処理ループ |
| `processor.go` | ZIP展開・WebP変換・PDF生成 |

## 設計上の決定事項

### DPI変換なし
PDFのページサイズはピクセル値をそのままポイント単位として使用する。厳密な印刷用途には `pixels × 72 / dpi` の換算が必要だが、現在はスクリーン表示用途のみを想定。

### WebP → PNG 変換
`gopdf` がWebPを直接サポートしないため、`golang.org/x/image/webp` でデコードして8-bit RGBA PNGとして一時保存する。gopdfが16-bit PNGも扱えないため、デコード後に `image/draw` で8-bit RGBAへ変換する。

### ZIP内重複ファイル名
異なるサブディレクトリに同名ファイルがある場合は `name_1.ext` のサフィックスを付与して重複を回避する（`uniqueName` 関数）。

### PDF出力先
ZIPファイルと同じディレクトリに `<zip名>.pdf` として出力する。

## 依存ライブラリ

| ライブラリ | 用途 |
|---|---|
| `github.com/signintech/gopdf` | PDF生成 |
| `golang.org/x/image` | WebPデコード |
