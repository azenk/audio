package stream

import (
	"context"
	"math"
)

func Sinewave(ctx context.Context, bufferLen int, amplitude, frequency, phase float64, sampleRate int) chan Sample {
	sampleCh := make(chan Sample, bufferLen)
	radsPerSec := 2 * math.Pi * frequency
	phaseRadians := phase * 2 * math.Pi / 360
	amplitude = amplitude * float64(math.MaxInt32)
	sampleInterval := 1 / float64(sampleRate)

	go func() {
		var v Sample
		var t float64
		for {
			v = Sample(math.Sin(radsPerSec*t+phaseRadians) * amplitude)
			t += sampleInterval
			select {
			case <-ctx.Done():
				close(sampleCh)
				return
			case sampleCh <- v:
			}
		}
	}()
	return sampleCh
}
