package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
)

func main() {
	var inputDir string
	flag.StringVar(&inputDir, "i", ".", "処理対象のディレクトリパス (デフォルト: カレントディレクトリ)")
	flag.StringVar(&inputDir, "input", ".", "処理対象のディレクトリパス (デフォルト: カレントディレクトリ) -iのエイリアス")
	flag.Parse()

	absPath, err := filepath.Abs(inputDir)
	if err != nil {
		log.Fatalf("入力ディレクトリの絶対パス取得に失敗しました: %v", err)
	}

	fmt.Printf("対象ディレクトリ: %s\n", absPath)

	// zipファイルの走査
	pattern := filepath.Join(absPath, "*.zip")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("zipファイルの検索中にエラーが発生しました: %v", err)
	}

	if len(matches) == 0 {
		fmt.Println("対象のzipファイルが見つかりませんでした。")
		return
	}

	fmt.Printf("%d件のzipファイルを見つけました。\n", len(matches))
	for _, zipFile := range matches {
		err := ProcessZip(zipFile)
		if err != nil {
			log.Printf("エラー (%s): %v\n", filepath.Base(zipFile), err)
		}
	}
}
