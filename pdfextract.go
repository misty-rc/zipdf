package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type pdfImageMeta struct {
	width       int
	height      int
	bpc         int
	colorSpace  string
	filter      string
	predictor   int
	length      int64
	streamStart int64
}

// extractPDFImagesDirect はPDFから画像ストリームを1枚ずつ直接読み込みます。
// FlateDecode（PNG predictor 10〜15）と DCTDecode に対応しています。
func extractPDFImagesDirect(pdfPath, destDir string) ([]string, error) {
	f, err := os.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("PDFのオープンに失敗しました: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	startXRef, err := pdfFindStartXRef(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("startxref の検索に失敗しました: %w", err)
	}

	objOffsets, err := pdfParseXRefTable(f, startXRef)
	if err != nil {
		return nil, fmt.Errorf("XRef テーブルの解析に失敗しました: %w", err)
	}

	type entry struct {
		objNr int
		path  string
	}
	var entries []entry

	for objNr, off := range objOffsets {
		meta, ok, err := pdfReadImageMeta(f, off)
		if err != nil || !ok {
			continue
		}

		var outPath string
		var extractErr error

		switch meta.filter {
		case "FlateDecode":
			outPath = filepath.Join(destDir, fmt.Sprintf("obj%07d.png", objNr))
			extractErr = pdfExtractFlateAsPNG(f, meta, outPath)
		case "DCTDecode":
			outPath = filepath.Join(destDir, fmt.Sprintf("obj%07d.jpg", objNr))
			extractErr = pdfExtractDCTAsJPEG(f, meta, outPath)
		default:
			continue
		}

		if extractErr != nil {
			return nil, fmt.Errorf("obj %d の画像抽出に失敗しました: %w", objNr, extractErr)
		}
		entries = append(entries, entry{objNr, outPath})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("PDFから画像を抽出できませんでした")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].objNr < entries[j].objNr })

	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.path
	}
	return paths, nil
}

// pdfFindStartXRef はファイル末尾 1024 バイトを読み startxref オフセットを返します。
func pdfFindStartXRef(f *os.File, fileSize int64) (int64, error) {
	const tailLen = 1024
	seekPos := fileSize - tailLen
	if seekPos < 0 {
		seekPos = 0
	}
	if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
		return 0, err
	}
	tail := make([]byte, fileSize-seekPos)
	if _, err := io.ReadFull(f, tail); err != nil {
		return 0, err
	}
	idx := bytes.LastIndex(tail, []byte("startxref"))
	if idx < 0 {
		return 0, fmt.Errorf("startxref が見つかりません")
	}
	rest := bytes.TrimLeft(tail[idx+len("startxref"):], " \t\r\n")
	if end := bytes.IndexAny(rest, " \t\r\n"); end > 0 {
		rest = rest[:end]
	}
	off, err := strconv.ParseInt(strings.TrimSpace(string(rest)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("startxref の値を解析できませんでした: %w", err)
	}
	return off, nil
}

// pdfParseXRefTable は従来形式（非ストリーム）の XRef テーブルをパースします。
func pdfParseXRefTable(f *os.File, offset int64) (map[int]int64, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	// 4MB は 200K オブジェクト分のテーブルをカバーする
	buf := make([]byte, 4*1024*1024)
	n, _ := f.Read(buf)
	buf = buf[:n]

	if !bytes.HasPrefix(bytes.TrimSpace(buf), []byte("xref")) {
		return nil, fmt.Errorf("XRef ストリーム形式 (PDF 1.5+) は非対応です")
	}

	result := make(map[int]int64)
	lines := bytes.Split(buf, []byte("\n"))
	i := 1 // "xref" 行をスキップ

	for i < len(lines) {
		line := bytes.TrimRight(lines[i], "\r")
		i++

		if bytes.Equal(bytes.TrimSpace(line), []byte("trailer")) {
			break
		}

		parts := bytes.Fields(line)
		if len(parts) != 2 {
			continue
		}
		firstObj, err1 := strconv.Atoi(string(parts[0]))
		count, err2 := strconv.Atoi(string(parts[1]))
		if err1 != nil || err2 != nil {
			continue
		}

		for j := 0; j < count && i < len(lines); j++ {
			entry := bytes.TrimRight(lines[i], "\r ")
			i++
			// "nnnnnnnnnn ggggg n" の形式: index 17 が 'n' or 'f'
			if len(entry) < 18 || entry[17] != 'n' {
				continue
			}
			off, err := strconv.ParseInt(string(entry[:10]), 10, 64)
			if err == nil && off > 0 {
				result[firstObj+j] = off
			}
		}
	}

	return result, nil
}

var (
	reSubtypeImage = regexp.MustCompile(`/Subtype\s*/Image`)
	reImageMask    = regexp.MustCompile(`/ImageMask\s+true`)
	reWidth        = regexp.MustCompile(`/Width\s+(\d+)`)
	reHeight       = regexp.MustCompile(`/Height\s+(\d+)`)
	reBPC          = regexp.MustCompile(`/BitsPerComponent\s+(\d+)`)
	reColorSpace   = regexp.MustCompile(`/ColorSpace\s+/(\w+)`)
	reFilter       = regexp.MustCompile(`/Filter\s*/(\w+)`)
	reLength       = regexp.MustCompile(`/Length\s+(\d+)\b`)
	rePredictor    = regexp.MustCompile(`/Predictor\s+(\d+)`)
)

// pdfReadImageMeta は指定オフセットのオブジェクトを読み画像メタデータを返します。
// 対応画像 XObject なら (meta, true, nil)、それ以外は (_, false, nil) を返します。
func pdfReadImageMeta(f *os.File, offset int64) (pdfImageMeta, bool, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return pdfImageMeta{}, false, nil
	}
	buf := make([]byte, 1024)
	n, _ := f.Read(buf)
	buf = buf[:n]

	// Truncate at "endobj" to avoid reading data from adjacent objects.
	// Short objects (pages, content streams) have endobj within a few hundred
	// bytes. Large image streams (~MB) won't have endobj in the first 1024 bytes.
	if idx := bytes.Index(buf, []byte("endobj")); idx >= 0 {
		buf = buf[:idx]
	}

	// Extract the dict portion (before "stream" keyword) and the stream start offset.
	var dictBuf []byte
	var streamStart int64
	if idx := bytes.Index(buf, []byte("stream\r\n")); idx >= 0 {
		dictBuf = buf[:idx]
		streamStart = offset + int64(idx) + 8
	} else if idx := bytes.Index(buf, []byte("stream\n")); idx >= 0 {
		dictBuf = buf[:idx]
		streamStart = offset + int64(idx) + 7
	} else {
		return pdfImageMeta{}, false, nil
	}

	if !reSubtypeImage.Match(dictBuf) || reImageMask.Match(dictBuf) {
		return pdfImageMeta{}, false, nil
	}

	meta := pdfImageMeta{streamStart: streamStart}

	if m := reWidth.FindSubmatch(dictBuf); m != nil {
		meta.width, _ = strconv.Atoi(string(m[1]))
	}
	if m := reHeight.FindSubmatch(dictBuf); m != nil {
		meta.height, _ = strconv.Atoi(string(m[1]))
	}
	if m := reBPC.FindSubmatch(dictBuf); m != nil {
		meta.bpc, _ = strconv.Atoi(string(m[1]))
	}
	if m := reColorSpace.FindSubmatch(dictBuf); m != nil {
		meta.colorSpace = string(m[1])
	}
	if m := reFilter.FindSubmatch(dictBuf); m != nil {
		meta.filter = string(m[1])
	}
	if m := reLength.FindSubmatch(dictBuf); m != nil {
		meta.length, _ = strconv.ParseInt(string(m[1]), 10, 64)
	}
	if m := rePredictor.FindSubmatch(dictBuf); m != nil {
		meta.predictor, _ = strconv.Atoi(string(m[1]))
	}

	if meta.width == 0 || meta.height == 0 || meta.length == 0 {
		return pdfImageMeta{}, false, nil
	}
	if meta.colorSpace != "DeviceRGB" && meta.colorSpace != "DeviceGray" {
		return pdfImageMeta{}, false, nil
	}
	// FlateDecode は PNG predictor (10-15) のみ対応
	if meta.filter == "FlateDecode" && (meta.predictor < 10 || meta.predictor > 15) {
		return pdfImageMeta{}, false, nil
	}

	return meta, true, nil
}

// pdfExtractFlateAsPNG は FlateDecode+PNG predictor ストリームを PNG コンテナに包んで保存します。
// Predictor 10〜15 のストリームバイトは PNG IDAT と同一フォーマットのため解凍不要です。
func pdfExtractFlateAsPNG(f *os.File, meta pdfImageMeta, outPath string) error {
	if _, err := f.Seek(meta.streamStart, io.SeekStart); err != nil {
		return err
	}
	compressed := make([]byte, meta.length)
	if _, err := io.ReadFull(f, compressed); err != nil {
		return fmt.Errorf("ストリームの読み込みに失敗しました: %w", err)
	}

	var colorType byte
	switch meta.colorSpace {
	case "DeviceRGB":
		colorType = 2
	case "DeviceGray":
		colorType = 0
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return pdfWriteSyntheticPNG(out, meta.width, meta.height, meta.bpc, colorType, compressed)
}

// pdfExtractDCTAsJPEG は DCTDecode ストリームを JPEG ファイルとして直接コピーします。
func pdfExtractDCTAsJPEG(f *os.File, meta pdfImageMeta, outPath string) error {
	if _, err := f.Seek(meta.streamStart, io.SeekStart); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.CopyN(out, f, meta.length)
	return err
}

// pdfWriteSyntheticPNG は FlateDecode の生バイトを IDAT チャンクとして使い PNG ファイルを書き出します。
func pdfWriteSyntheticPNG(w io.Writer, width, height, bpc int, colorType byte, data []byte) error {
	if _, err := w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10}); err != nil {
		return err
	}

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = byte(bpc)
	ihdr[9] = colorType
	if err := pdfWritePNGChunk(w, "IHDR", ihdr); err != nil {
		return err
	}

	const maxChunk = 65535
	for len(data) > 0 {
		sz := len(data)
		if sz > maxChunk {
			sz = maxChunk
		}
		if err := pdfWritePNGChunk(w, "IDAT", data[:sz]); err != nil {
			return err
		}
		data = data[sz:]
	}

	return pdfWritePNGChunk(w, "IEND", nil)
}

func pdfWritePNGChunk(w io.Writer, typ string, data []byte) error {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	if _, err := w.Write([]byte(typ)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	h := crc32.NewIEEE()
	h.Write([]byte(typ))
	h.Write(data)
	crcBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBuf, h.Sum32())
	_, err := w.Write(crcBuf)
	return err
}
