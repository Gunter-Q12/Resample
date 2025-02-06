package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

type wavHeader struct {
	FileTypeBlockId [4]byte
	FileSize        uint32
	FileFormatId    [4]byte

	FormatBlockId [4]byte
	BlockSize     uint32
	AudioFormat   uint16
	NbrChannels   uint16
	Frequency     uint32
	BytePerSec    uint32
	BytePerBlock  uint16
	BitsPerSample uint16

	DataBlockId [4]byte
	DataSize    uint32
}

var defaultHeader = wavHeader{
	FileTypeBlockId: [4]byte{'R', 'I', 'F', 'F'},
	FileFormatId:    [4]byte{'W', 'A', 'V', 'E'},
	FormatBlockId:   [4]byte{'f', 'm', 't', ' '},
	DataBlockId:     [4]byte{'d', 'a', 't', 'a'},
	BlockSize:       16, // 18 and 40-bit headers are not supported
}

func readHeader(f *os.File) (rate, ch int, format string, err error) {
	var header wavHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return 0, 0, "", err
	}

	rate = int(header.Frequency)
	ch = int(header.NbrChannels)
	bitsPerSample := strconv.Itoa(int(header.BitsPerSample))

	if header.AudioFormat == 1 {
		return rate, ch, "i" + bitsPerSample, nil
	}
	if header.AudioFormat == 3 {
		return rate, ch, "f" + bitsPerSample, nil
	}

	return
}

func writeHeader(f *os.File, rate, ch int, format string) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()

	var audioFormat uint16
	switch format[0] {
	case 'i':
		audioFormat = 1
	case 'f':
		audioFormat = 3
	default:
		return fmt.Errorf("unknown audio format %d", format[0])
	}

	bitsPerSample, err := strconv.Atoi(format[1:])
	if err != nil {
		return fmt.Errorf("incorrect number of bits per sample %d", format[0])
	}

	header := defaultHeader
	header.FileSize = uint32(size - 8)
	header.AudioFormat = audioFormat
	header.NbrChannels = uint16(ch)
	header.Frequency = uint32(rate)
	header.BitsPerSample = uint16(bitsPerSample)
	header.BytePerBlock = uint16(ch * bitsPerSample / 8)
	header.BytePerSec = uint32(header.BytePerBlock) * header.Frequency
	header.DataSize = uint32(size - 44)

	return binary.Write(f, binary.LittleEndian, &header)
}
