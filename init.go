package imgsz

import "io"

func init() {
	RegisterFormat("jpeg", "\xff\xd8", func(r io.Reader) (Size, error) {
		var d jpgdecoder
		return d.decode(r)
	})
	RegisterFormat("png", pngHeader, decodepng)
	RegisterFormat("gif", "GIF8?a", func(r io.Reader) (Size, error) {
		var d gifdecoder
		if err := d.readHeaderAndScreenDescriptor(r); err != nil {
			return Size{}, err
		}
		return Size{d.width, d.height}, nil
	})
	RegisterFormat("webp", "RIFF????WEBPVP8", decodewebp)
}
