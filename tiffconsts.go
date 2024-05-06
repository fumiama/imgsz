// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package imgsz

// A tiff image file contains one or more images. The metadata
// of each image is contained in an Image File Directory (IFD),
// which contains entries of 12 bytes each and is described
// on page 14-16 of the specification. An IFD entry consists of
//
//  - a tag, which describes the signification of the entry,
//  - the data type and length of the entry,
//  - the data itself or a pointer to it if it is more than 4 bytes.
//
// The presence of a length means that each IFD is effectively an array.

const (
	leHeader = "II\x2A\x00" // Header for little-endian files.
	beHeader = "MM\x00\x2A" // Header for big-endian files.

	ifdLen = 12 // Length of an IFD entry in bytes.
)

// Data types (p. 14-16 of the spec).
const (
	dtByte     = 1
	dtASCII    = 2
	dtShort    = 3
	dtLong     = 4
	dtRational = 5
)

// The length of one instance of each data type in bytes.
var lengths = [...]uint32{0, 1, 1, 2, 4, 8}

// Tags (see p. 28-41 of the spec).
const (
	tImageWidth                = 256
	tImageLength               = 257
	tBitsPerSample             = 258
	tCompression               = 259
	tPhotometricInterpretation = 262

	tFillOrder = 266

	tStripOffsets    = 273
	tSamplesPerPixel = 277
	tRowsPerStrip    = 278
	tStripByteCounts = 279

	tT4Options = 292 // CCITT Group 3 options, a set of 32 flag bits.
	tT6Options = 293 // CCITT Group 4 options, a set of 32 flag bits.

	tTileWidth      = 322
	tTileLength     = 323
	tTileOffsets    = 324
	tTileByteCounts = 325

	tPredictor    = 317
	tColorMap     = 320
	tExtraSamples = 338
	tSampleFormat = 339
)
