// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gif implements a GIF image gifdecoder and encoder.
//
// The GIF specification is at https://www.w3.org/Graphics/GIF/spec-gif89a.txt.

package imgsz

import (
	"fmt"
	"io"
)

func readFull(r io.Reader, b []byte) error {
	_, err := io.ReadFull(r, b)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// gifdecoder is the type used to decode a GIF file.
type gifdecoder struct {
	// From header.
	vers   string
	width  int
	height int

	// Used when decoding.
	tmp [1024]byte // must be at least 768 so we can read color table
}

func (d *gifdecoder) readHeaderAndScreenDescriptor(r io.Reader) error {
	err := readFull(r, d.tmp[:13])
	if err != nil {
		return fmt.Errorf("gif: reading header: %v", err)
	}
	d.vers = string(d.tmp[:6])
	if d.vers != "GIF87a" && d.vers != "GIF89a" {
		return fmt.Errorf("gif: can't recognize format %q", d.vers)
	}
	d.width = int(d.tmp[6]) + int(d.tmp[7])<<8
	d.height = int(d.tmp[8]) + int(d.tmp[9])<<8
	// d.tmp[12] is the Pixel Aspect Ratio, which is ignored.
	return nil
}
