package imgsz

import (
	"os"
	"testing"
)

func TestSizes(t *testing.T) {
	f, err := os.Open("testdata/test.webp")
	if err != nil {
		t.Fatal(err)
	}
	sz, n, err := DecodeSize(f)
	if err != nil {
		t.Fatal(err)
	}
	if sz.Width != 3507 || sz.Height != 2480 || n != "webp" {
		t.Fatal(sz, n)
	}
	f.Close()

	f, err = os.Open("testdata/test.jpg")
	if err != nil {
		t.Fatal(err)
	}
	sz, n, err = DecodeSize(f)
	if err != nil {
		t.Fatal(err)
	}
	if sz.Width != 858 || sz.Height != 1126 || n != "jpeg" {
		t.Fatal(sz, n)
	}
	f.Close()

	f, err = os.Open("testdata/test.png")
	if err != nil {
		t.Fatal(err)
	}
	sz, n, err = DecodeSize(f)
	if err != nil {
		t.Fatal(err)
	}
	if sz.Width != 670 || sz.Height != 717 || n != "png" {
		t.Fatal(sz, n)
	}
	f.Close()

	f, err = os.Open("testdata/test.gif")
	if err != nil {
		t.Fatal(err)
	}
	sz, n, err = DecodeSize(f)
	if err != nil {
		t.Fatal(err)
	}
	if sz.Width != 184 || sz.Height != 166 || n != "gif" {
		t.Fatal(sz, n)
	}
	f.Close()
}
