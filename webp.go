// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package imgsz

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"math"
)

var errInvalidFormat = errors.New("webp: invalid format")

// fourCC is a four character code.
type fourCC [4]byte

var (
	fccALPH = fourCC{'A', 'L', 'P', 'H'}
	fccVP8  = fourCC{'V', 'P', '8', ' '}
	fccVP8L = fourCC{'V', 'P', '8', 'L'}
	fccVP8X = fourCC{'V', 'P', '8', 'X'}
	fccWEBP = fourCC{'W', 'E', 'B', 'P'}
)

const chunkHeaderSize = 8

var (
	errMissingPaddingByte     = errors.New("riff: missing padding byte")
	errMissingRIFFChunkHeader = errors.New("riff: missing RIFF chunk header")
	errListSubchunkTooLong    = errors.New("riff: list subchunk too long")
	errShortChunkData         = errors.New("riff: short chunk data")
	errShortChunkHeader       = errors.New("riff: short chunk header")
	errStaleReader            = errors.New("riff: stale reader")
)

// u32 decodes the first four bytes of b as a little-endian integer.
func u32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// webpeader reads chunks from an underlying io.webpeader.
type webpeader struct {
	r   io.Reader
	err error

	totalLen uint32
	chunkLen uint32

	chunkReader *chunkReader
	buf         [chunkHeaderSize]byte
	padded      bool
}

// newListReader returns a LIST chunk's list type, such as "movi" or "wavl",
// and its chunks as a *Reader.
func newListReader(chunkLen uint32, chunkData io.Reader) (listType fourCC, data *webpeader, err error) {
	if chunkLen < 4 {
		return fourCC{}, nil, errShortChunkData
	}
	z := &webpeader{r: chunkData}
	if _, err := io.ReadFull(chunkData, z.buf[:4]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err = errShortChunkData
		}
		return fourCC{}, nil, err
	}
	z.totalLen = chunkLen - 4
	return fourCC{z.buf[0], z.buf[1], z.buf[2], z.buf[3]}, z, nil
}

// newReader returns the RIFF stream's form type, such as "AVI " or "WAVE", and
// its chunks as a *Reader.
func newReader(r io.Reader) (formType fourCC, data *webpeader, err error) {
	var buf [chunkHeaderSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err = errMissingRIFFChunkHeader
		}
		return fourCC{}, nil, err
	}
	if buf[0] != 'R' || buf[1] != 'I' || buf[2] != 'F' || buf[3] != 'F' {
		return fourCC{}, nil, errMissingRIFFChunkHeader
	}
	return newListReader(u32(buf[4:]), r)
}

// next returns the next chunk's ID, length and data. It returns io.EOF if there
// are no more chunks. The io.Reader returned becomes stale after the next next
// call, and should no longer be used.
//
// It is valid to call next even if all of the previous chunk's data has not
// been read.
func (z *webpeader) next() (chunkID fourCC, chunkLen uint32, chunkData io.Reader, err error) {
	if z.err != nil {
		return fourCC{}, 0, nil, z.err
	}

	// Drain the rest of the previous chunk.
	if z.chunkLen != 0 {
		want := z.chunkLen
		var got int64
		got, z.err = io.Copy(ioutil.Discard, z.chunkReader)
		if z.err == nil && uint32(got) != want {
			z.err = errShortChunkData
		}
		if z.err != nil {
			return fourCC{}, 0, nil, z.err
		}
	}
	z.chunkReader = nil
	if z.padded {
		if z.totalLen == 0 {
			z.err = errListSubchunkTooLong
			return fourCC{}, 0, nil, z.err
		}
		z.totalLen--
		_, z.err = io.ReadFull(z.r, z.buf[:1])
		if z.err != nil {
			if z.err == io.EOF {
				z.err = errMissingPaddingByte
			}
			return fourCC{}, 0, nil, z.err
		}
	}

	// We are done if we have no more data.
	if z.totalLen == 0 {
		z.err = io.EOF
		return fourCC{}, 0, nil, z.err
	}

	// Read the next chunk header.
	if z.totalLen < chunkHeaderSize {
		z.err = errShortChunkHeader
		return fourCC{}, 0, nil, z.err
	}
	z.totalLen -= chunkHeaderSize
	if _, z.err = io.ReadFull(z.r, z.buf[:chunkHeaderSize]); z.err != nil {
		if z.err == io.EOF || z.err == io.ErrUnexpectedEOF {
			z.err = errShortChunkHeader
		}
		return fourCC{}, 0, nil, z.err
	}
	chunkID = fourCC{z.buf[0], z.buf[1], z.buf[2], z.buf[3]}
	z.chunkLen = u32(z.buf[4:])
	if z.chunkLen > z.totalLen {
		z.err = errListSubchunkTooLong
		return fourCC{}, 0, nil, z.err
	}
	z.padded = z.chunkLen&1 == 1
	z.chunkReader = &chunkReader{z}
	return chunkID, z.chunkLen, z.chunkReader, nil
}

type chunkReader struct {
	z *webpeader
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if c != c.z.chunkReader {
		return 0, errStaleReader
	}
	z := c.z
	if z.err != nil {
		if z.err == io.EOF {
			return 0, errStaleReader
		}
		return 0, z.err
	}

	n := int(z.chunkLen)
	if n == 0 {
		return 0, io.EOF
	}
	if n < 0 {
		// Converting uint32 to int overflowed.
		n = math.MaxInt32
	}
	if n > len(p) {
		n = len(p)
	}
	n, err := z.r.Read(p[:n])
	z.totalLen -= uint32(n)
	z.chunkLen -= uint32(n)
	if err != io.EOF {
		z.err = err
	}
	return n, err
}

func decodewebp(r io.Reader) (Size, error) {
	formType, riffReader, err := newReader(r)
	if err != nil {
		return Size{}, err
	}
	if formType != fccWEBP {
		return Size{}, errInvalidFormat
	}

	var (
		alpha          []byte
		wantAlpha      bool
		widthMinusOne  uint32
		heightMinusOne uint32
		buf            [10]byte
	)
	for {
		chunkID, chunkLen, chunkData, err := riffReader.next()
		if err == io.EOF {
			err = errInvalidFormat
		}
		if err != nil {
			return Size{}, err
		}

		switch chunkID {
		case fccALPH:
			if !wantAlpha {
				return Size{}, errInvalidFormat
			}
			wantAlpha = false
			// Read the Pre-processing | Filter | Compression byte.
			if _, err := io.ReadFull(chunkData, buf[:1]); err != nil {
				if err == io.EOF {
					err = errInvalidFormat
				}
				return Size{}, err
			}

		case fccVP8:
			if wantAlpha || int32(chunkLen) < 0 {
				return Size{}, errInvalidFormat
			}
			w, h, err := decodeVP8FrameHeader(chunkData)
			if err != nil {
				return Size{}, err
			}
			return Size{w, h}, nil

		case fccVP8L:
			if wantAlpha || alpha != nil {
				return Size{}, errInvalidFormat
			}
			w, h, err := decodeVP8LHeader(chunkData)
			return Size{int(w), int(h)}, err

		case fccVP8X:
			if chunkLen != 10 {
				return Size{}, errInvalidFormat
			}
			if _, err := io.ReadFull(chunkData, buf[:10]); err != nil {
				return Size{}, err
			}
			const (
				animationBit    = 1 << 1
				xmpMetadataBit  = 1 << 2
				exifMetadataBit = 1 << 3
				alphaBit        = 1 << 4
				iccProfileBit   = 1 << 5
			)
			wantAlpha = (buf[0] & alphaBit) != 0
			widthMinusOne = uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16
			heightMinusOne = uint32(buf[7]) | uint32(buf[8])<<8 | uint32(buf[9])<<16
			if wantAlpha {
				return Size{
					Width:  int(widthMinusOne) + 1,
					Height: int(heightMinusOne) + 1,
				}, nil
			}
			return Size{
				Width:  int(widthMinusOne) + 1,
				Height: int(heightMinusOne) + 1,
			}, nil
		}
	}
}

func decodeVP8FrameHeader(r io.Reader) (w, h int, err error) {
	var scratch [8]byte
	// All frame headers are at least 3 bytes long.
	b := scratch[:3]
	if _, err = io.ReadFull(r, b); err != nil {
		return
	}
	if (b[0] & 1) != 0 {
		return 0, 0, nil
	}
	// Frame headers for key frames are an additional 7 bytes long.
	b = scratch[:7]
	if _, err = io.ReadFull(r, b); err != nil {
		return
	}
	// Check the magic sync code.
	if b[0] != 0x9d || b[1] != 0x01 || b[2] != 0x2a {
		err = errors.New("vp8: invalid format")
		return
	}
	return int(b[4]&0x3f)<<8 | int(b[3]), int(b[6]&0x3f)<<8 | int(b[5]), nil
}

// vp8ldecoder holds the bit-stream for a VP8L image.
type vp8ldecoder struct {
	r     io.ByteReader
	bits  uint32
	nBits uint32
}

// read reads the next n bits from the decoder's bit-stream.
func (d *vp8ldecoder) read(n uint32) (uint32, error) {
	for d.nBits < n {
		c, err := d.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		d.bits |= uint32(c) << d.nBits
		d.nBits += 8
	}
	u := d.bits & (1<<n - 1)
	d.bits >>= n
	d.nBits -= n
	return u, nil
}

func decodeVP8LHeader(r io.Reader) (w int32, h int32, err error) {
	rr, ok := r.(io.ByteReader)
	if !ok {
		rr = bufio.NewReader(r)
	}
	d := &vp8ldecoder{r: rr}
	magic, err := d.read(8)
	if err != nil {
		return 0, 0, err
	}
	if magic != 0x2f {
		return 0, 0, errors.New("vp8l: invalid header")
	}
	width, err := d.read(14)
	if err != nil {
		return 0, 0, err
	}
	width++
	height, err := d.read(14)
	if err != nil {
		return 0, 0, err
	}
	height++
	_, err = d.read(1) // Read and ignore the hasAlpha hint.
	if err != nil {
		return 0, 0, err
	}
	version, err := d.read(3)
	if err != nil {
		return 0, 0, err
	}
	if version != 0 {
		return 0, 0, errors.New("vp8l: invalid version")
	}
	return int32(width), int32(height), nil
}
