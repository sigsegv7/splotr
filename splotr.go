package main

import (
	"fmt"
	"os"
)

//
// The MP3 frame header exists before every
// frame present in the file.
//
//
// @Emphasis
//    Tells us that the file must be de-emphasized based
//    on the following:
//        0b00 - none
//        0b01 - 50/15 ms
//        0b10 - reserved
//        0b11 - CCIT J.17
//
// @Original
//     Tells us if this file is a copy or original media
//     based on the following:
//         0b0 - Copy of original media
//         0b1 - Original media
//
//
// @Copyright
//     Tells us if the file is copyrighted media based on
//     the following:
//         0b0 - Audio is not copyrighted media
//         0b1 - Audio is copyrighted media
//
//
// @ModeExt (only used in joint stereo)
//     Mode extension is a space optimization used to reduce
//     the size of frames by multiplexing data for joint stereo
//     based on the following:
//     -----------------------------------------------------
//     |value|----|Layer I & II|----|Intensity|----|MS|----
//      0b00       bands 4 - 31         off         off
//      0b01       bands 8 - 31         on          off
//      0b10       bands 12 - 31        off         on
//      0b11       bands 16 - 31        on          on
//     -----------------------------------------------------
//
//
// @ChannelMode
//     Describes the channel modes based on the following:
//         0b00 - Stereo
//         0b01 - Joint stereo
//         0b10 - Dual [mono] channel
//         0b11 - Mono
//
// @IsPadded
//     Padding is used to ensure that the frame fits the bitrate.
//     This field describes if the frame is padded based on the following:
//         0b0 - Not padded
//         0b1 - Padded
//
// @Srfi (Sampling rate frequency index)
//     |value|------|mpgeg1|------|mpeg2|-----|mpeg2.5|------
//      0b00         44100 hz      22050 hz    11025 hz
//      0b01         48000 hz      24000 hz    12000 hz
//      0b10         32000 hz      16000 hz    8000 hz
//      0b11         reserved      reserved    reserved
//     ------------------------------------------------------
//
// @Protection bit
//     Describes if the frame is protected by CRC based
//     on the following:
//         0b0 - Protected by CRC (CRC follows header, 16 bits)
//         0b1 - Not protected by CRC
//
// @LayerDesc
//     Layer description is as follows:
//         0b00 - Reserved
//         0b01 - Layer III
//         0b10 - Layer II
//         0b11 - Layer I
//
// @FrameSync
//     Set to all 1s. This is used by us to find the
//     start of the frame header.
//
//
type Mp3FrameHeader struct {
	Emphasis        uint8  // 1 bit
	Original        uint8  // 1 bit
	Copyright       uint8  // 1 bit
	ModeExt         uint8  // 2 bits
	ChannelMode     uint8  // 2 bits
	Unused          uint8  // 1 bit
	IsPadded        uint8  // 1 bit
	Srfi            uint8  // 2 bits
	BitrateIdx      uint8  // 4 bits
	CrcProtected    uint8  // 1 bit
	LayerDesc       uint8  // 2 bits
	AudioVer        uint8  // 2 bits
	FrameSync       uint16 // 11 bits
}

type Mp3File struct {
	Path            Mp3Path  // Path of .mp3 file
	DurationMin     Mp3Dur   // Duration in min
	DurationSec     Mp3Dur   // Duration in sec
	Size            Mp3Size  // Size of file in bytes
	Contents        []byte   // Raw binary contents
}

// Common types
type Mp3EStream    []byte		// Stream of bytes (elementary stream)
type Mp3Frame      Mp3EStream	// MP3 frame
type Mp3Path       Mp3EStream	// MP3 path (i.e., filepath)
type Mp3Dur        uint32       // Duration
type Mp3Size       int64

func Load(path Mp3Path) (*Mp3File, error) {
	var handle *Mp3File = new(Mp3File)
	var filesize Mp3Size
	var origstr string
	var copyrstr string
	data, err := os.ReadFile(string(path))

	if err != nil {
		fmt.Println("error: failed to open", path)
		return nil, err
	}

	// If the file is not empty then we'll strip the
	// trailing newline
	if len(data) > 0 {
		data[len(data) - 1] = '\x00'
	}

	handle.Path = Mp3Path(path)
	handle.DurationMin = 0
	handle.DurationSec = 0

	// Attempt to fetch how big the file is.
	filesize, err = GetFileSize(handle.Path)
	if err != nil {
		return nil, err
	}

	handle.Size = filesize
	handle.Contents = data
	frame := DeserializeFrame(Mp3Frame(data))

	// Is this an original copy?
	if frame.Original == 1 {
		origstr = "yes"
	} else {
		origstr = "no"
	}

	// Is this copyrighted media?
	if frame.Copyright == 1 {
		copyrstr = "yes"
	} else {
		copyrstr = "no"
	}

	fmt.Println("Emphasis: ", frame.Emphasis)
	fmt.Println("Original copy: ", origstr)
	fmt.Println("Is copyrighted: ", copyrstr)
	return handle, nil
}

// Go does not support struct packing in the way
// that C does... However, we can work around this
// by writing routines that allow you to deserialize
// it.
func DeserializeFrame(fr Mp3Frame) Mp3FrameHeader {
	var b0 uint8 = fr[0]
	var b1 uint8 = fr[1]
	var b2 uint8 = fr[2]
	var b3 uint8 = fr[3]
	var hdr *Mp3FrameHeader = new(Mp3FrameHeader)

	// b0 extraction
	hdr.Emphasis = b0 & 3
	hdr.Original = (b0 >> 2) & 1
	hdr.Copyright = (b0 >> 3) & 1
	hdr.ModeExt = (b0 >> 4) & 3
	hdr.ChannelMode = (b0 >> 6) & 3

	// b1 extraction
	hdr.Unused = b1 & 1
	hdr.IsPadded = (b1 >> 1) & 1
	hdr.Srfi = (b1 >> 2) & 3
	hdr.BitrateIdx = (b1 >> 4) & 0xF

	// b2 extraction
	hdr.CrcProtected = b2 & 1
	hdr.LayerDesc = (b2 >> 1) & 3
	hdr.AudioVer = (b2 >> 3) & 3

	// Extract final bits
	hdr.FrameSync |= ((uint16(b2) >> 5) | (uint16(b3) << 3))
	return *hdr
}

func GetFileSize(path Mp3Path) (Mp3Size, error) {
	f, err := os.Open(string(path))
	st, err := f.Stat()
	if err != nil {
		fmt.Println("internal error: Failed to get file size")
		return -1, err
	}

	return Mp3Size(st.Size()), nil
}


// Prints the banner for when the program starts...
// Should have been self explanatory though ::)
func Banner() {
	fmt.Println("Splotr v1.0 - sound plotter")
	fmt.Println("Written by Ian M. Moffett")
}

func main() {
	Banner()
	if len(os.Args) < 2 {
		panic("too few arguments!")
	}

	path := Mp3Path(os.Args[1])
	_, err := Load(path)

	if err != nil {
		fmt.Println("Could not find ", string(path))
		panic("bailing")
	}
}
