# imgsz
Image size analyzer for jpg/png/gif/webp

# Usage

```go
// DecodeSize decodes the dimensions of an image that has
// been encoded in a registered format. The string returned is the format name
// used during format registration. Format registration is typically done by
// an init function in the codec-specific package.
func DecodeSize(r io.Reader) (Size, string, error)
```
