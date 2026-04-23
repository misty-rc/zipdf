package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var inputDir string
	flag.StringVar(&inputDir, "i", ".", "処理対象のディレクトリパス (デフォルト: カレントディレクトリ)")
	flag.StringVar(&inputDir, "input", ".", "処理対象のディレクトリパス (-i のエイリアス)")

	var quality int
	flag.IntVar(&quality, "q", 85, "JPEG再エンコード品質 (1-100, デフォルト: 85)")
	flag.IntVar(&quality, "quality", 85, "JPEG再エンコード品質 (-q のエイリアス)")

	var noCompress bool
	flag.BoolVar(&noCompress, "no-compress", false, "再エンコードを無効にして元画像をそのまま埋め込む")

	var recompress bool
	flag.BoolVar(&recompress, "recompress", false, "既存PDFの埋め込み画像を再エンコードして再PDF化する")

	var override bool
	flag.BoolVar(&override, "override", false, "再PDF化時に元ファイルを上書きする (--recompress 時のみ有効)")

	flag.Parse()

	if override && !recompress {
		log.Fatalf("--override は --recompress と組み合わせて使用してください")
	}
	if quality < 1 || quality > 100 {
		log.Fatalf("--quality は 1〜100 の範囲で指定してください: %d", quality)
	}
	if noCompress {
		quality = 0
	}

	absPath, err := filepath.Abs(inputDir)
	if err != nil {
		log.Fatalf("入力ディレクトリの絶対パス取得に失敗しました: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Fatalf("ディレクトリが存在しません: %s", absPath)
	}

	fmt.Printf("対象ディレクトリ: %s\n", absPath)

	if recompress {
		if quality > 0 {
			fmt.Printf("モード: PDF再圧縮 (JPEG品質 %d)\n", quality)
		} else {
			fmt.Println("モード: PDF再圧縮 (再エンコードなし)")
		}

		matches, err := filepath.Glob(filepath.Join(absPath, "*.pdf"))
		if err != nil {
			log.Fatalf("PDFファイルの検索中にエラーが発生しました: %v", err)
		}
		if len(matches) == 0 {
			fmt.Println("対象のPDFファイルが見つかりませんでした。")
			return
		}
		var targets []string
		for _, p := range matches {
			if !strings.HasSuffix(strings.TrimSuffix(p, filepath.Ext(p)), "_compressed") {
				targets = append(targets, p)
			}
		}
		fmt.Printf("%d件のPDFファイルを見つけました。\n", len(targets))
		for _, pdfFile := range targets {
			if err := ProcessPDF(pdfFile, quality, override); err != nil {
				log.Printf("エラー (%s): %v", filepath.Base(pdfFile), err)
			}
		}
		return
	}

	if quality > 0 {
		fmt.Printf("JPEG再エンコード: 品質 %d\n", quality)
	} else {
		fmt.Println("JPEG再エンコード: 無効")
	}

	matches, err := filepath.Glob(filepath.Join(absPath, "*.zip"))
	if err != nil {
		log.Fatalf("zipファイルの検索中にエラーが発生しました: %v", err)
	}
	if len(matches) == 0 {
		fmt.Println("対象のzipファイルが見つかりませんでした。")
		return
	}

	fmt.Printf("%d件のzipファイルを見つけました。\n", len(matches))
	for _, zipFile := range matches {
		if err := ProcessZip(zipFile, quality); err != nil {
			log.Printf("エラー (%s): %v", filepath.Base(zipFile), err)
		}
	}
}
