package media

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// durationMS reports how long a clip is without decoding a single sample, so a
// library of feature-length videos still opens instantly. Anything the probes do
// not recognise comes back as 0 and the tile simply omits the length.
func durationMS(path string, kind Kind) int64 {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v", ".mov", ".m4a":
		if ms, err := isoDuration(path); err == nil {
			return ms
		}
	case ".wav":
		if ms, err := wavDuration(path); err == nil {
			return ms
		}
	case ".mp3":
		if ms, err := mp3Duration(path); err == nil {
			return ms
		}
	}
	return 0
}

// isoDuration walks the container for moov > mvhd, which every QuickTime-derived
// file writes with the whole presentation's timescale and duration.
func isoDuration(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return scanAtoms(file, 0, info.Size(), 0)
}

var containerAtoms = map[string]bool{
	"moov": true, "trak": true, "mdia": true, "minf": true, "stbl": true,
}

func scanAtoms(file *os.File, start, end int64, depth int) (int64, error) {
	if depth > 4 {
		return 0, nil
	}

	header := make([]byte, 16)
	for offset := start; offset+8 <= end; {
		if _, err := file.ReadAt(header[:8], offset); err != nil {
			return 0, err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		atom := string(header[4:8])
		used := int64(8)
		switch size {
		case 1:
			if _, err := file.ReadAt(header[8:16], offset+8); err != nil {
				return 0, err
			}
			size = int64(binary.BigEndian.Uint64(header[8:16]))
			used = 16
		case 0:
			size = end - offset
		}
		if size < used {
			return 0, fmt.Errorf("malformed %s atom", atom)
		}

		body := offset + used
		switch {
		case atom == "mvhd":
			return mvhdDuration(file, body)
		case containerAtoms[atom]:
			if ms, err := scanAtoms(file, body, offset+size, depth+1); err == nil && ms > 0 {
				return ms, nil
			}
		}
		offset += size
	}
	return 0, nil
}

func mvhdDuration(file *os.File, body int64) (int64, error) {
	head := make([]byte, 32)
	if _, err := file.ReadAt(head, body); err != nil {
		return 0, err
	}

	var timescale, duration int64
	if head[0] == 1 {
		timescale = int64(binary.BigEndian.Uint32(head[20:24]))
		duration = int64(binary.BigEndian.Uint64(head[24:32]))
	} else {
		timescale = int64(binary.BigEndian.Uint32(head[12:16]))
		duration = int64(binary.BigEndian.Uint32(head[16:20]))
	}
	if timescale <= 0 || duration <= 0 {
		return 0, fmt.Errorf("unreadable mvhd timing")
	}
	return duration * 1000 / timescale, nil
}

// wavDuration divides the data chunk by the byte rate recorded in fmt.
func wavDuration(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	riff := make([]byte, 12)
	if _, err := io.ReadFull(file, riff); err != nil {
		return 0, err
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a wav file")
	}

	chunk := make([]byte, 8)
	var byteRate int64
	for {
		if _, err := io.ReadFull(file, chunk); err != nil {
			return 0, err
		}
		name := string(chunk[0:4])
		size := int64(binary.LittleEndian.Uint32(chunk[4:8]))

		if name == "fmt " {
			format := make([]byte, 16)
			if _, err := io.ReadFull(file, format); err != nil {
				return 0, err
			}
			byteRate = int64(binary.LittleEndian.Uint32(format[8:12]))
			if _, err := file.Seek(size-16, io.SeekCurrent); err != nil {
				return 0, err
			}
			continue
		}
		if name == "data" && byteRate > 0 {
			return size * 1000 / byteRate, nil
		}
		if _, err := file.Seek(size, io.SeekCurrent); err != nil {
			return 0, err
		}
	}
}

var (
	mpeg1Bitrates = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	mpeg2Bitrates = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
)

// mp3Duration assumes the constant bitrate most worship backing tracks are
// exported at, which is close enough for a grid tile and never wrong by minutes.
func mp3Duration(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	head := make([]byte, 4)
	for offset := int64(0); offset+4 <= info.Size(); offset++ {
		if _, err := file.ReadAt(head, offset); err != nil {
			return 0, err
		}
		frame := binary.BigEndian.Uint32(head)
		if frame&0xFFE00000 != 0xFFE00000 {
			continue
		}
		version := byte((frame >> 19) & 0x03)
		layer := byte((frame >> 17) & 0x03)
		bitrateIndex := int((frame >> 12) & 0x0F)
		rateIndex := int((frame >> 10) & 0x03)
		if version == 1 || layer != 1 || rateIndex == 3 || bitrateIndex == 0 || bitrateIndex == 15 {
			continue
		}

		bitrates := mpeg2Bitrates
		if version == 3 {
			bitrates = mpeg1Bitrates
		}
		kbps := int64(bitrates[bitrateIndex])
		if kbps == 0 {
			continue
		}
		return info.Size() * 8000 / kbps, nil
	}
	return 0, fmt.Errorf("no mpeg frame header")
}
