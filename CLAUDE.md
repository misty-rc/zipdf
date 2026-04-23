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

# 既存PDFを再圧縮（_compressed.pdf として出力）
./zipdf --recompress -i ./pdfs -q 70

# 既存PDFを再圧縮して上書き
./zipdf --recompress --override -i ./pdfs -q 70
```

## フラグ一覧

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-i`, `--input` | `.` | 処理対象ディレクトリ |
| `-q`, `--quality` | `85` | JPEG再エンコード品質（1〜100） |
| `--no-compress` | `false` | 再エンコードを無効化し元画像をそのまま埋め込む |
| `--recompress` | `false` | 既存PDFの埋め込み画像を再エンコードして再PDF化する |
| `--override` | `false` | `--recompress` 時に元ファイルを上書きする（単独使用不可） |

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | フラグ解析・ディレクトリスキャン・処理ループ |
| `processor.go` | ZIP展開・WebP変換・JPEG再エンコード・PDF生成・PDF再圧縮 |
| `pdfextract.go` | XRefテーブル直接パース・画像ストリーム抽出（`--recompress` 用） |

## 処理フロー

### ZIPモード（デフォルト）

```
ZIP展開（sequential）
  ↓
ファイル名順ソート
  ↓
JPEG再エンコード（parallel: runtime.NumCPU() ワーカー）  ← --no-compress 時はスキップ
  ↓
PDF生成（sequential）
```

### PDF再圧縮モード（--recompress）

```
XRefテーブル直接パース → 画像 XObject を ObjNr 昇順で抽出（sequential）→ 一時ディレクトリ
  ↓
JPEG再エンコード（parallel: runtime.NumCPU() ワーカー）  ← --no-compress 時はスキップ
  ↓
PDF生成（sequential）
  ↓
--override 時: 一時ファイルへ書き込み後 os.Rename で上書き
```

`*_compressed.pdf` はスキャン対象から除外する（再処理防止）。

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

### PDF再圧縮の出力
- デフォルト: `<元ファイル名>_compressed.pdf`（元ファイル保持）
- `--override` 指定時: 同ディレクトリの一時ファイルに書き込んでから `os.Rename` で上書き（読み書き競合回避）

### PDF直接パーサー（pdfextract.go）
pdfcpu を使わず XRefテーブルを直接パースし、画像ストリームを1枚ずつ読み込む。
- 対応フィルタ: `FlateDecode`（PNG predictor 10〜15）、`DCTDecode`
- `FlateDecode + Predictor 15` は PDF のストリームバイト列が PNG IDAT と同一フォーマットのため、解凍せず PNG コンテナに包むだけで `image/png` で読み込める
- 対応カラースペース: `DeviceRGB`、`DeviceGray`
- `endobj` でバッファを切り詰め、隣接オブジェクトを誤検知しないようにしている

## 依存ライブラリ

| ライブラリ | 用途 |
|---|---|
| `github.com/signintech/gopdf` | PDF生成 |
| `golang.org/x/image` | WebPデコード |
