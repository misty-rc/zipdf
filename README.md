# zipdf

ZIPアーカイブ内の画像をファイル名順でPDFに結合するコマンドラインツール。

## 目的

- ZIP内の画像（jpg/jpeg/png/webp）をファイル名順に並べてPDF化する
- 複数のZIPを一括処理し、1 ZIP → 1 PDF を出力する
- コマンドラインで利用することを前提とする

## インストール

```bash
git clone https://github.com/misty-rc/zipdf
cd zipdf
go build -o zipdf.exe .
```

## 使い方

```bash
# 指定ディレクトリ内の *.zip をすべて処理（デフォルト品質85でJPEG再エンコード）
./zipdf -i ./archive

# 品質を変えて処理
./zipdf -i ./archive -q 70

# 再エンコードなし（元画像をそのまま埋め込む）
./zipdf -i ./archive --no-compress

# カレントディレクトリを処理
./zipdf

# 既存PDFを再圧縮（<元ファイル名>_compressed.pdf として出力）
./zipdf --recompress -i ./pdfs -q 70

# 既存PDFを再圧縮して上書き
./zipdf --recompress --override -i ./pdfs -q 70
```

## フラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-i`, `--input` | `.`（カレント） | 処理対象ディレクトリ |
| `-q`, `--quality` | `85` | JPEG再エンコード品質（1〜100）|
| `--no-compress` | `false` | 再エンコードを無効化し元画像をそのまま埋め込む |
| `--recompress` | `false` | 既存PDFの埋め込み画像を再エンコードして再PDF化する |
| `--override` | `false` | `--recompress` 時に元ファイルを上書きする（単独使用不可） |

## 対応フォーマット

`.jpg` / `.jpeg` / `.png` / `.webp`

## 出力

ZIPと同じディレクトリに `<ZIPファイル名>.pdf` を生成する。

## 技術スタック

- Go
- [gopdf](https://github.com/signintech/gopdf) — PDF生成
- [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) — WebPデコード
