package pt

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

func showProgress(start time.Time, rays uint64, i, h int) {
	pct := int(100 * float64(i) / float64(h))
	elapsed := time.Since(start)
	rps := float64(rays) / elapsed.Seconds()
	fmt.Printf("\r%4d / %d (%3d%%) [", i, h, pct)
	for p := 0; p < 100; p += 3 {
		if pct > p {
			fmt.Print("=")
		} else {
			fmt.Print(" ")
		}
	}
	fmt.Printf("] %s %s ", DurationString(elapsed), NumberString(rps))
}

func Render(scene *Scene, camera *Camera, w, h, cameraSamples, hitSamples, bounces int) image.Image {
	ncpu := runtime.NumCPU()
	runtime.GOMAXPROCS(ncpu)
	scene.Compile()
	result := image.NewNRGBA(image.Rect(0, 0, w, h))
	ch := make(chan int, h)
	absCameraSamples := int(math.Abs(float64(cameraSamples)))
	fmt.Printf("%d x %d pixels, %d x %d = %d samples, %d bounces, %d cores\n",
		w, h, absCameraSamples, hitSamples, absCameraSamples*hitSamples, bounces, ncpu)
	start := time.Now()
	scene.rays = 0
	for i := 0; i < ncpu; i++ {
		go func(i int) {
			rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
			for y := i; y < h; y += ncpu {
				for x := 0; x < w; x++ {
					c := Color{}
					if cameraSamples <= 0 {
						// random subsampling
						for i := 0; i < absCameraSamples; i++ {
							fu := rnd.Float64()
							fv := rnd.Float64()
							ray := camera.CastRay(x, y, w, h, fu, fv)
							c = c.Add(scene.Sample(ray, true, hitSamples, bounces, rnd))
						}
						c = c.DivScalar(float64(absCameraSamples))
					} else {
						// stratified subsampling
						n := int(math.Sqrt(float64(cameraSamples)))
						for u := 0; u < n; u++ {
							for v := 0; v < n; v++ {
								fu := (float64(u) + 0.5) / float64(n)
								fv := (float64(v) + 0.5) / float64(n)
								ray := camera.CastRay(x, y, w, h, fu, fv)
								c = c.Add(scene.Sample(ray, true, hitSamples, bounces, rnd))
							}
						}
						c = c.DivScalar(float64(n * n))
					}
					c = c.Pow(1 / 2.2)
					r := uint8(math.Min(255, c.R*255))
					g := uint8(math.Min(255, c.G*255))
					b := uint8(math.Min(255, c.B*255))
					result.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
				}
				ch <- 1
			}
		}(i)
	}
	showProgress(start, scene.RayCount(), 0, h)
	for i := 0; i < h; i++ {
		<-ch
		showProgress(start, scene.RayCount(), i+1, h)
	}
	fmt.Println()
	return result
}

func IterativeRender(pathTemplate string, iterations int, scene *Scene, camera *Camera, w, h, cameraSamples, hitSamples, bounces int) error {
	scene.Compile()
	pixels := make([]Color, w*h)
	result := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 1; i <= iterations; i++ {
		fmt.Printf("\n[Iteration %d of %d]\n", i, iterations)
		frame := Render(scene, camera, w, h, cameraSamples, hitSamples, bounces)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				index := y*w + x
				r, g, b, _ := frame.At(x, y).RGBA()
				c := Color{float64(r) / 65535, float64(g) / 65535, float64(b) / 65535}
				pixels[index] = pixels[index].Add(c)
				avg := pixels[index].DivScalar(float64(i))
				ar := uint8(math.Min(255, avg.R*255))
				ag := uint8(math.Min(255, avg.G*255))
				ab := uint8(math.Min(255, avg.B*255))
				result.SetNRGBA(x, y, color.NRGBA{ar, ag, ab, 255})
			}
		}
		path := pathTemplate
		if strings.Contains(path, "%") {
			path = fmt.Sprintf(pathTemplate, i)
		}
		if err := SavePNG(path, result); err != nil {
			return err
		}
	}
	return nil
}
