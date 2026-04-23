# zipdf

ZIPアーカイブ内の画像をファイル名順でPDFに結合するコマンドラインツール。

## ビルド

```bash
go build -o zipdf.exe .
```

## 使い方

```bash
# デフォルト（品質85でJPEG再エンコード）
./zipdf -i ./archive

# 品質を指定
./zipdf -i ./archive -q 70

# 再エンコードなし（元画像をそのまま埋め込む）
./zipdf -i ./archive --no-compress

# カレントディレクトリを処理
./zipdf
```

## フラグ一覧

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-i`, `--input` | `.` | 処理対象ディレクトリ |
| `-q`, `--quality` | `85` | JPEG再エンコード品質（1〜100） |
| `--no-compress` | `false` | 再エンコードを無効化し元画像をそのまま埋め込む |

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | フラグ解析・ディレクトリスキャン・処理ループ |
| `processor.go` | ZIP展開・WebP変換・JPEG再エンコード・PDF生成 |

## 処理フロー

```
ZIP展開（sequential）
  ↓
ファイル名順ソート
  ↓
JPEG再エンコード（parallel: runtime.NumCPU() ワーカー）  ← --no-compress 時はスキップ
  ↓
PDF生成（sequential）
```

### imageInfo 構造体

再エンコード後の `path`・`width`・`height` をまとめて渡す。`generatePDF` は `[]imageInfo` を受け取り `getImageDimensions` を呼ばない。

## 設計上の決定事項

### JPEG再エンコード（デフォルト有効・品質85）
PNG/WebP由来ページのサイズ削減が主目的。再エンコード時に画像をフルデコードするため `getImageDimensions` の呼び出しが不要になり、ファイルopenを1回削減できる。アルファチャンネルは白背景に合成してからエンコードする（JPEG非対応のため）。

### WebP → PNG 変換
`gopdf` がWebPを直接サポートしないため、`golang.org/x/image/webp` でデコードして8-bit RGBA PNGとして一時保存する。gopdfが16-bit PNGも扱えないため、`image/draw` で8-bit RGBAへ変換する。再エンコードが有効な場合はこのPNGがさらにJPEGに変換される。

### DPI変換なし
PDFのページサイズはピクセル値をそのままポイント単位として使用する。厳密な印刷用途には `pixels × 72 / dpi` の換算が必要だが、現在はスクリーン表示用途のみを想定。

### ZIP内重複ファイル名
異なるサブディレクトリに同名ファイルがある場合は `name_1.ext` のサフィックスを付与して重複を回避する（`uniqueName` 関数）。生成名も `seen` マップに登録し、元ファイル名との衝突も防止する。

### PDF出力先
ZIPファイルと同じディレクトリに `<zip名>.pdf` として出力する。

## 依存ライブラリ

| ライブラリ | 用途 |
|---|---|
| `github.com/signintech/gopdf` | PDF生成 |
| `golang.org/x/image` | WebPデコード |
