// Command mkassets writes the iPad app's asset catalog: its icon, the
// Boing Ball before the 1984 demo's purple grid (an iOS icon has no
// transparency), and its launch screen's ball and background colour.
//
//	go run ./internal/boing/mkassets ios/BlitterBox/Assets.xcassets
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/andrewsjg/blitterbox/internal/boing"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mkassets DIR.xcassets")
		os.Exit(2)
	}
	dir := os.Args[1]
	write(filepath.Join(dir, "Contents.json"), catalog)
	write(filepath.Join(dir, "AppIcon.appiconset", "Contents.json"), appIcon)
	writePNG(filepath.Join(dir, "AppIcon.appiconset", "icon-1024.png"), icon(1024))
	write(filepath.Join(dir, "LaunchBall.imageset", "Contents.json"), launchBall)
	writePNG(filepath.Join(dir, "LaunchBall.imageset", "ball@2x.png"), boing.Ball(320))
	write(filepath.Join(dir, "LaunchBackground.colorset", "Contents.json"), launchBackground)
}

// icon is the ball before the demo's grid: a wall of squares, and a
// floor running back to it in perspective.
func icon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0xB4, 0xB4, 0xB8, 0xFF}
	line := color.RGBA{0x9A, 0x2E, 0xA8, 0xFF}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	s := float64(size)
	cell := s / 12
	width := s / 170
	horizon := s * 0.78 // where the wall meets the floor
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			var d float64 // distance to the nearest grid line, in pixels
			if fy < horizon {
				d = math.Min(dist(fx, cell), dist(fy, cell))
			} else {
				// The floor: lines running back to a vanishing point
				// above the middle, and across, closer together further off.
				depth := (fy - horizon) / (s - horizon) // 0 at the wall, 1 at the front
				spread := 0.35 + 0.65*depth
				u := (fx - s/2) / spread
				v := math.Sqrt(depth) * 4
				d = math.Min(dist(u+s/2, cell)*spread, dist(v, 1)*(s-horizon)/4)
				if fy-horizon < width { // the wall's foot
					d = 0
				}
			}
			if a := 1 - math.Max(0, math.Min(1, d-width/2+0.5)); a > 0 {
				img.Set(x, y, mix(bg, line, a))
			}
		}
	}
	ball := boing.Ball(int(s * 0.76))
	off := image.Pt((size-ball.Bounds().Dx())/2, int(s*0.07))
	draw.Draw(img, ball.Bounds().Add(off), ball, image.Point{}, draw.Over)
	return img
}

// dist is how far v is from the nearest multiple of step.
func dist(v, step float64) float64 {
	m := math.Mod(math.Mod(v, step)+step, step) // left of zero too
	return math.Min(m, step-m)
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 0xFF}
}

func writePNG(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	if err := png.Encode(f, img); err != nil {
		fail(err)
	}
	if err := f.Close(); err != nil {
		fail(err)
	}
}

func write(path, text string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkassets:", err)
	os.Exit(1)
}

const info = `"info" : { "author" : "xcode", "version" : 1 }`

var (
	catalog = "{\n  " + info + "\n}\n"
	appIcon = `{
  "images" : [
    { "filename" : "icon-1024.png", "idiom" : "universal", "platform" : "ios", "size" : "1024x1024" }
  ],
  ` + info + `
}
`
	launchBall = `{
  "images" : [
    { "filename" : "ball@2x.png", "idiom" : "universal", "scale" : "2x" }
  ],
  ` + info + `
}
`
	// The status bar's dark grey (amiga/display_status.go), dark or light.
	launchBackground = `{
  "colors" : [
    { "color" : { "color-space" : "srgb", "components" : { "red" : "0x1C", "green" : "0x1D", "blue" : "0x20", "alpha" : "1.000" } }, "idiom" : "universal" }
  ],
  ` + info + `
}
`
)
