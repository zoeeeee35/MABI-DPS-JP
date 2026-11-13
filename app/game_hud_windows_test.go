//go:build native && windows

package main

import "testing"

func TestDetectGameHUD(t *testing.T) {
	const width, height = 250, 140
	pixels := make([]uint32, width*height)
	if _, valid := detectGameHUD(pixels, width, height); valid {
		t.Fatal("black capture treated as a valid sample")
	}
	for i := range pixels {
		pixels[i] = 0x354355
	}
	for y := 64; y < 72; y++ {
		for x := 90; x < 182; x++ {
			pixels[y*width+x] = 0x33D85A
		}
	}
	if visible, valid := detectGameHUD(pixels, width, height); visible || !valid {
		t.Fatal("green bar alone matched")
	}
	for y := 38; y < 56; y++ {
		for x := 14; x < 17; x++ {
			pixels[y*width+x] = 0x4DC9FC
		}
	}
	if visible, valid := detectGameHUD(pixels, width, height); !visible || !valid {
		t.Fatal("bar beside portrait rim did not match")
	}
}

func TestLoadingScreenTextOnDarkBackground(t *testing.T) {
	pixels := make([]uint32, 1000)
	if hasLoadingScreenText(pixels) {
		t.Fatal("black capture mistaken for a loading title")
	}
	for i := 0; i < 50; i++ {
		pixels[i] = 0xFFFFFF
	}
	if !hasLoadingScreenText(pixels) {
		t.Fatal("bright loading title on dark background was not detected")
	}
	for i := 50; i < 500; i++ {
		pixels[i] = 0x405060
	}
	if hasLoadingScreenText(pixels) {
		t.Fatal("bright text on a non-dark background mistaken for a loading title")
	}
}
