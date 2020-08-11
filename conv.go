package main

import (
	"image/color"
	"math"
)

func coltof3(c color.Color) [3]float32 {
	r, g, b, _ := c.RGBA()
	return [3]float32{
		float32(r) / 65535.0,
		float32(g) / 65535.0,
		float32(b) / 65535.0,
	}
}

func ftou8(f float32) uint8 {
	f = clamp32(f, 0.0, 1.0)
	if f == 1.0 {
		return 255
	}
	return uint8(math.Floor(float64(f * 256.0)))
}

func f3tocol(c [3]float32) color.RGBA {
	return color.RGBA{
		ftou8(c[0]),
		ftou8(c[1]),
		ftou8(c[2]),
		0xff,
	}
}

func clamp32(v, min, max float32) float32 {
	return float32(clamp(float64(v), float64(min), float64(max)))
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
