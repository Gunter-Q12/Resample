package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

const (
	byteSize = 8

	wavHeaderSize       = 44
	wavBlockSize        = 16 // 18 and 40-bit headers are not supported
	wavIntAudioFormat   = 1
	wavFloatAudioFormat = 3
)

type wavHeader struct {
	FileTypeBlockID [4]byte
	FileSize        uint32
	FileFormatID    [4]byte

	FormatBlockID [4]byte
	BlockSize     uint32
	AudioFormat   uint16
	NbrChannels   uint16
	Frequency     uint32
	BytePerSec    uint32
	BytePerBlock  uint16
	BitsPerSample uint16

	DataBlockID [4]byte
	DataSize    uint32
}

var defaultHeader = wavHeader{
	FileTypeBlockID: [4]byte{'R', 'I', 'F', 'F'},
	FileFormatID:    [4]byte{'W', 'A', 'V', 'E'},
	FormatBlockID:   [4]byte{'f', 'm', 't', ' '},
	DataBlockID:     [4]byte{'d', 'a', 't', 'a'},
	BlockSize:       wavBlockSize,
}

func readHeader(f *os.File) (rate, ch int, format string, err error) {
	var header wavHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return 0, 0, "", err
	}

	rate = int(header.Frequency)
	ch = int(header.NbrChannels)
	bitsPerSample := strconv.Itoa(int(header.BitsPerSample))

	switch header.AudioFormat {
	case wavIntAudioFormat:
		return rate, ch, "i" + bitsPerSample, nil
	case wavFloatAudioFormat:
		return rate, ch, "f" + bitsPerSample, nil
	}
	return 0, 0, "", fmt.Errorf("unknown audio format: %d", header.AudioFormat)
}

//nolint:gosec // unsafe work with bytes
func writeHeader(f *os.File, rate, ch int, format string) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()

	var audioFormat uint16
	switch format[0] {
	case 'i':
		audioFormat = wavIntAudioFormat
	case 'f':
		audioFormat = wavFloatAudioFormat
	default:
		return fmt.Errorf("unknown audio format %d", format[0])
	}

	bitsPerSample, err := strconv.Atoi(format[1:])
	if err != nil {
		return fmt.Errorf("incorrect number of bits per sample %d", format[0])
	}

	header := defaultHeader
	header.FileSize = uint32(size - 8) //nolint:mnd // I do not know why there is an 8
	header.AudioFormat = audioFormat
	header.NbrChannels = uint16(ch)
	header.Frequency = uint32(rate)
	header.BitsPerSample = uint16(bitsPerSample)
	header.BytePerBlock = uint16(ch * bitsPerSample / byteSize)
	header.BytePerSec = uint32(header.BytePerBlock) * header.Frequency
	header.DataSize = uint32(size - wavHeaderSize)

	return binary.Write(f, binary.LittleEndian, &header)
}
