// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tiff implements a TIFF image tiffdecoder and encoder.
//
// The TIFF specification is at http://partners.adobe.com/public/developer/en/tiff/TIFF6.pdf
package imgsz // import "golang.org/x/image/tiff"

import (
	"encoding/binary"
	"image/color"
	"io"
	"math"
)

const maxChunkSize = 10 << 20 // 10M

// safeReadAt is a verbatim copy of internal/saferio.ReadDataAt from the
// standard library, which is used to read data from a reader using a length
// provided by untrusted data, without allocating the entire slice ahead of time
// if it is large (>maxChunkSize). This allows us to avoid allocating giant
// slices before learning that we can't actually read that much data from the
// reader.
func safeReadAt(r io.ReaderAt, n uint64, off int64) ([]byte, error) {
	if int64(n) < 0 || n != uint64(int(n)) {
		// n is too large to fit in int, so we can't allocate
		// a buffer large enough. Treat this as a read failure.
		return nil, io.ErrUnexpectedEOF
	}

	if n < maxChunkSize {
		buf := make([]byte, n)
		_, err := r.ReadAt(buf, off)
		if err != nil {
			// io.SectionReader can return EOF for n == 0,
			// but for our purposes that is a success.
			if err != io.EOF || n > 0 {
				return nil, err
			}
		}
		return buf, nil
	}

	var buf []byte
	buf1 := make([]byte, maxChunkSize)
	for n > 0 {
		next := n
		if next > maxChunkSize {
			next = maxChunkSize
		}
		_, err := r.ReadAt(buf1[:next], off)
		if err != nil {
			return nil, err
		}
		buf = append(buf, buf1[:next]...)
		n -= next
		off += int64(next)
	}
	return buf, nil
}

type tiffdecoder struct {
	r         io.ReaderAt
	byteOrder binary.ByteOrder
	config    Size
	features  map[int][]uint
	palette   []color.Color
}

// firstVal returns the first uint of the features entry with the given tag,
// or 0 if the tag does not exist.
func (d *tiffdecoder) firstVal(tag int) uint {
	f := d.features[tag]
	if len(f) == 0 {
		return 0
	}
	return f[0]
}

// ifdUint decodes the IFD entry in p, which must be of the Byte, Short
// or Long type, and returns the decoded uint values.
func (d *tiffdecoder) ifdUint(p []byte) (u []uint, err error) {
	var raw []byte
	if len(p) < ifdLen {
		return nil, FormatError("bad IFD entry")
	}

	datatype := d.byteOrder.Uint16(p[2:4])
	if dt := int(datatype); dt <= 0 || dt >= len(lengths) {
		return nil, UnsupportedError("IFD entry datatype")
	}

	count := d.byteOrder.Uint32(p[4:8])
	if count > math.MaxInt32/lengths[datatype] {
		return nil, FormatError("IFD data too large")
	}
	if datalen := lengths[datatype] * count; datalen > 4 {
		// The IFD contains a pointer to the real value.
		raw, err = safeReadAt(d.r, uint64(datalen), int64(d.byteOrder.Uint32(p[8:12])))
	} else {
		raw = p[8 : 8+datalen]
	}
	if err != nil {
		return nil, err
	}

	u = make([]uint, count)
	switch datatype {
	case dtByte:
		for i := uint32(0); i < count; i++ {
			u[i] = uint(raw[i])
		}
	case dtShort:
		for i := uint32(0); i < count; i++ {
			u[i] = uint(d.byteOrder.Uint16(raw[2*i : 2*(i+1)]))
		}
	case dtLong:
		for i := uint32(0); i < count; i++ {
			u[i] = uint(d.byteOrder.Uint32(raw[4*i : 4*(i+1)]))
		}
	default:
		return nil, UnsupportedError("data type")
	}
	return u, nil
}

// parseIFD decides whether the IFD entry in p is "interesting" and
// stows away the data in the tiffdecoder. It returns the tag number of the
// entry and an error, if any.
func (d *tiffdecoder) parseIFD(p []byte) (int, error) {
	tag := d.byteOrder.Uint16(p[0:2])
	switch tag {
	case tBitsPerSample,
		tExtraSamples,
		tPhotometricInterpretation,
		tCompression,
		tPredictor,
		tStripOffsets,
		tStripByteCounts,
		tRowsPerStrip,
		tTileWidth,
		tTileLength,
		tTileOffsets,
		tTileByteCounts,
		tImageLength,
		tImageWidth,
		tFillOrder,
		tT4Options,
		tT6Options:
		val, err := d.ifdUint(p)
		if err != nil {
			return 0, err
		}
		d.features[int(tag)] = val
	case tColorMap:
		val, err := d.ifdUint(p)
		if err != nil {
			return 0, err
		}
		numcolors := len(val) / 3
		if len(val)%3 != 0 || numcolors <= 0 || numcolors > 256 {
			return 0, FormatError("bad ColorMap length")
		}
		d.palette = make([]color.Color, numcolors)
		for i := 0; i < numcolors; i++ {
			d.palette[i] = color.RGBA64{
				uint16(val[i]),
				uint16(val[i+numcolors]),
				uint16(val[i+2*numcolors]),
				0xffff,
			}
		}
	case tSampleFormat:
		// Page 27 of the spec: If the SampleFormat is present and
		// the value is not 1 [= unsigned integer data], a Baseline
		// TIFF reader that cannot handle the SampleFormat value
		// must terminate the import process gracefully.
		val, err := d.ifdUint(p)
		if err != nil {
			return 0, err
		}
		for _, v := range val {
			if v != 1 {
				return 0, UnsupportedError("sample format")
			}
		}
	}
	return int(tag), nil
}

func newtiffDecoder(r io.Reader) (*tiffdecoder, error) {
	d := &tiffdecoder{
		r:        newReaderAt(r),
		features: make(map[int][]uint),
	}

	p := make([]byte, 8)
	if _, err := d.r.ReadAt(p, 0); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	switch string(p[0:4]) {
	case leHeader:
		d.byteOrder = binary.LittleEndian
	case beHeader:
		d.byteOrder = binary.BigEndian
	default:
		return nil, FormatError("malformed header")
	}

	ifdOffset := int64(d.byteOrder.Uint32(p[4:8]))

	// The first two bytes contain the number of entries (12 bytes each).
	if _, err := d.r.ReadAt(p[0:2], ifdOffset); err != nil {
		return nil, err
	}
	numItems := int(d.byteOrder.Uint16(p[0:2]))

	// All IFD entries are read in one chunk.
	var err error
	p, err = safeReadAt(d.r, uint64(ifdLen*numItems), ifdOffset+2)
	if err != nil {
		return nil, err
	}

	prevTag := -1
	for i := 0; i < len(p); i += ifdLen {
		tag, err := d.parseIFD(p[i : i+ifdLen])
		if err != nil {
			return nil, err
		}
		if tag <= prevTag {
			return nil, FormatError("tags are not sorted in ascending order")
		}
		prevTag = tag
	}

	d.config.Width = int(d.firstVal(tImageWidth))
	d.config.Height = int(d.firstVal(tImageLength))

	return d, nil
}

// decodetiff returns the color model and dimensions of a TIFF image without
// decoding the entire image.
func decodetiff(r io.Reader) (Size, error) {
	d, err := newtiffDecoder(r)
	if err != nil {
		return Size{}, err
	}
	return d.config, nil
}
