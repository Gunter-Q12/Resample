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

	wf.file.Seek(4, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(size-8))

	wf.file.Seek(40, io.SeekStart)
	binary.Write(wf.file, binary.LittleEndian, int32(size-44))

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

	if strings.ToLower(filepath.Ext(inPath)) == ".wav" {
		header := make([]byte, 44)
		_, err = io.ReadFull(in, header)
		if err != nil {
			return nil, nil, err
		}

		_, err := out.Write(header)
		if err != nil {
			return nil, nil, err
		}

		out.Seek(24, io.SeekStart)
		binary.Write(out, binary.LittleEndian, int32(outRate))
		out.Seek(44, io.SeekStart)

		return in, &waveFile{file: out, header: header}, nil
	}

	return in, out, nil
}
