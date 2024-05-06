// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bmp implements a BMP image decoder and encoder.
//
// The BMP specification is at http://www.digicamsoft.com/bmp/bmp.html.
package imgsz // import "golang.org/x/image/bmp"

import (
	"errors"
	"image/color"
	"io"
)

// ErrUnsupported means that the input BMP image uses a valid but unsupported
// feature.
var ErrUnsupported = errors.New("bmp: unsupported BMP image")

func readUint16(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

func readUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func decodebmp(r io.Reader) (size Size, err error) {
	// We only support those BMP images with one of the following DIB headers:
	// - BITMAPINFOHEADER (40 bytes)
	// - BITMAPV4HEADER (108 bytes)
	// - BITMAPV5HEADER (124 bytes)
	const (
		fileHeaderLen   = 14
		infoHeaderLen   = 40
		v4InfoHeaderLen = 108
		v5InfoHeaderLen = 124
	)
	var b [1024]byte
	if _, err := io.ReadFull(r, b[:fileHeaderLen+4]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return Size{}, err
	}
	if string(b[:2]) != "BM" {
		return Size{}, errors.New("bmp: invalid format")
	}
	offset := readUint32(b[10:14])
	infoLen := readUint32(b[14:18])
	if infoLen != infoHeaderLen && infoLen != v4InfoHeaderLen && infoLen != v5InfoHeaderLen {
		return Size{}, ErrUnsupported
	}
	if _, err := io.ReadFull(r, b[fileHeaderLen+4:fileHeaderLen+infoLen]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return Size{}, err
	}
	width := int(int32(readUint32(b[18:22])))
	height := int(int32(readUint32(b[22:26])))
	if height < 0 {
		height = -height
	}
	if width < 0 || height < 0 {
		return Size{}, ErrUnsupported
	}
	// We only support 1 plane and 8, 24 or 32 bits per pixel and no
	// compression.
	planes, bpp, compression := readUint16(b[26:28]), readUint16(b[28:30]), readUint32(b[30:34])
	// if compression is set to BI_BITFIELDS, but the bitmask is set to the default bitmask
	// that would be used if compression was set to 0, we can continue as if compression was 0
	if compression == 3 && infoLen > infoHeaderLen &&
		readUint32(b[54:58]) == 0xff0000 && readUint32(b[58:62]) == 0xff00 &&
		readUint32(b[62:66]) == 0xff && readUint32(b[66:70]) == 0xff000000 {
		compression = 0
	}
	if planes != 1 || compression != 0 {
		return Size{}, ErrUnsupported
	}
	switch bpp {
	case 8:
		colorUsed := readUint32(b[46:50])
		// If colorUsed is 0, it is set to the maximum number of colors for the given bpp, which is 2^bpp.
		if colorUsed == 0 {
			colorUsed = 256
		} else if colorUsed > 256 {
			return Size{}, ErrUnsupported
		}

		if offset != fileHeaderLen+infoLen+colorUsed*4 {
			return Size{}, ErrUnsupported
		}
		_, err = io.ReadFull(r, b[:colorUsed*4])
		if err != nil {
			return Size{}, err
		}
		pcm := make(color.Palette, colorUsed)
		for i := range pcm {
			// BMP images are stored in BGR order rather than RGB order.
			// Every 4th byte is padding.
			pcm[i] = color.RGBA{b[4*i+2], b[4*i+1], b[4*i+0], 0xFF}
		}
		return Size{Width: width, Height: height}, nil
	case 24:
		if offset != fileHeaderLen+infoLen {
			return Size{}, ErrUnsupported
		}
		return Size{Width: width, Height: height}, nil
	case 32:
		if offset != fileHeaderLen+infoLen {
			return Size{}, ErrUnsupported
		}
		// 32 bits per pixel is possibly RGBX (X is padding) or RGBA (A is
		// alpha transparency). However, for BMP images, "Alpha is a
		// poorly-documented and inconsistently-used feature" says
		// https://source.chromium.org/chromium/chromium/src/+/bc0a792d7ebc587190d1a62ccddba10abeea274b:third_party/blink/renderer/platform/image-decoders/bmp/bmp_image_reader.cc;l=621
		//
		// That goes on to say "BITMAPV3HEADER+ have an alpha bitmask in the
		// info header... so we respect it at all times... [For earlier
		// (smaller) headers we] ignore alpha in Windows V3 BMPs except inside
		// ICO files".
		//
		// "Ignore" means to always set alpha to 0xFF (fully opaque):
		// https://source.chromium.org/chromium/chromium/src/+/bc0a792d7ebc587190d1a62ccddba10abeea274b:third_party/blink/renderer/platform/image-decoders/bmp/bmp_image_reader.h;l=272
		//
		// Confusingly, "Windows V3" does not correspond to BITMAPV3HEADER, but
		// instead corresponds to the earlier (smaller) BITMAPINFOHEADER:
		// https://source.chromium.org/chromium/chromium/src/+/bc0a792d7ebc587190d1a62ccddba10abeea274b:third_party/blink/renderer/platform/image-decoders/bmp/bmp_image_reader.cc;l=258
		//
		// This Go package does not support ICO files and the (infoLen >
		// infoHeaderLen) condition distinguishes BITMAPINFOHEADER (40 bytes)
		// vs later (larger) headers.
		return Size{Width: width, Height: height}, nil
	}
	return Size{}, ErrUnsupported
}
