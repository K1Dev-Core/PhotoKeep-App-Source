package photokeep

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestAnalyzeColors(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 45, G: 170, B: 75, A: 255})
		}
	}
	tags := analyzeColors(img)
	if !strings.Contains(strings.Join(tags, ","), "โทนเขียว") {
		t.Fatalf("color tags: %v", tags)
	}
}
