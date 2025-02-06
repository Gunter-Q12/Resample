package main

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type waveFile struct {
	file   *os.File
	header []byte

	outRate int
}

func (wf *waveFile) Write(p []byte) (n int, err error) {
	return wf.file.Write(p)
}

func (wf *waveFile) Close() error {
	defer wf.file.Close()

	info, err := wf.file.Stat()
	if err != nil {
		return err
	}
	size := info.Size()

	// FileSize
	wf.file.Seek(4, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(size-8))

	// Frequency
	wf.file.Seek(24, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(wf.outRate))

	// DataSize
	wf.file.Seek(40, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(size-44))

	var bytePerBlock int16
	wf.file.Seek(32, io.SeekStart)
	binary.Read(wf.file, binary.LittleEndian, &bytePerBlock)

	// BytePerSec
	wf.file.Seek(28, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(bytePerBlock)*int32(wf.outRate))

	return nil
}

func getInOutFiles(inPath, outPath string, outRate int) (io.ReadCloser, io.WriteCloser, error) {
	in, err := os.Open(inPath)
	if err != nil {
		return nil, nil, err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return nil, nil, err
	}

	header := make([]byte, 44)
	if strings.ToLower(filepath.Ext(inPath)) == ".wav" {
		_, err = io.ReadFull(in, header)
		if err != nil {
			return nil, nil, err
		}
	}
	if strings.ToLower(filepath.Ext(outPath)) == ".wav" {
		_, err := out.Write(header)
		if err != nil {
			return nil, nil, err
		}

		out.Seek(44, io.SeekStart)

		return in, &waveFile{
			file:    out,
			header:  header,
			outRate: outRate,
		}, nil
	}

	return in, out, nil
}
