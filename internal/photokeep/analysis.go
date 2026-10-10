package photokeep

import (
	"image"
	"path/filepath"
	"strings"
)

func initialTags(p Photo) string {
	tags := []string{strings.TrimPrefix(strings.ToUpper(filepath.Ext(p.Filename)), "."), p.TakenAt.Format("2006")}
	if p.Width > p.Height {
		tags = append(tags, "แนวนอน")
	} else if p.Height > p.Width {
		tags = append(tags, "แนวตั้ง")
	} else {
		tags = append(tags, "จัตุรัส")
	}
	return strings.Join(tags, ",")
}

func analyzeColors(img image.Image) []string {
	b := img.Bounds()
	stepX, stepY := maxInt(1, b.Dx()/80), maxInt(1, b.Dy()/80)
	counts := map[string]int{}
	var samples, brightness int
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r16, g16, bl16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 {
				continue
			}
			r, g, bl := int(r16>>8), int(g16>>8), int(bl16>>8)
			brightness += (r + g + bl) / 3
			samples++
			switch {
			case r > 160 && g > 140 && bl < 130 && r > bl+45 && g > bl+35:
				counts["โทนเหลือง"]++
			case r > g+45 && g > bl+20:
				counts["โทนส้ม"]++
			case r > g+45 && r > bl+45:
				counts["โทนแดง"]++
			case g > r+20 && g > bl+15:
				counts["โทนเขียว"]++
			case bl > r+20 && bl > g+12:
				counts["โทนฟ้า"]++
			}
		}
	}
	if samples == 0 {
		return nil
	}
	best, count := "", 0
	for name, n := range counts {
		if n > count {
			best, count = name, n
		}
	}
	var tags []string
	if count*20 >= samples {
		tags = append(tags, best)
	}
	average := brightness / samples
	if average >= 190 {
		tags = append(tags, "ภาพสว่าง")
	} else if average <= 70 {
		tags = append(tags, "ภาพมืด")
	}
	return tags
}
