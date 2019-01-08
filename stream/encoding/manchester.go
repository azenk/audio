package encoding

import (
	"context"
	"fmt"
	"math"

	"github.com/azenk/audio/stream"
)

type sampleClock struct {
	baseSamplePeriod int
	remainder        int64
	errMax           int64
	errPerCycle      int64
}

func newSampleClock(frequency float64, sampleRate int) *sampleClock {
	c := &sampleClock{}
	samplesPerCycle := float64(sampleRate) / frequency
	c.baseSamplePeriod = int(samplesPerCycle)
	c.errMax = 1e7
	c.errPerCycle = int64(math.Round((samplesPerCycle - math.Floor(samplesPerCycle)) * float64(c.errMax)))
	return c
}

// samples returns the number of samples in the high and low periods of this sample clock
func (c *sampleClock) samples() (int, int) {
	c1 := c.baseSamplePeriod>>1 + c.baseSamplePeriod%2
	c2 := c.baseSamplePeriod >> 1

	c.remainder += c.errPerCycle
	if c.remainder >= c.errMax {
		c2 = c2 + 1
		c.remainder = c.remainder - c.errMax
	}

	return c1, c2
}

func (c sampleClock) String() string {
	return fmt.Sprintf("Base Period: %d, Remainder: %d, ErrMax: %d, ErrPerCycle: %d", c.baseSamplePeriod, c.remainder, c.errMax, c.errPerCycle)
}

func DifferentialManchester(ctx context.Context, bufLen int, bitsPerSecond, amplitude float64, sampleRate int, inCh chan byte) chan stream.Sample {
	outCh := make(chan stream.Sample, bufLen)
	clock := newSampleClock(bitsPerSecond, sampleRate)
	go func() {
		var currentValue int32 = int32(amplitude * float64(math.MaxInt32))
		for byte := range inCh {
			for bit := 0; bit < 8; bit++ {
				bitValue := (byte >> uint(7-bit%8)) & 0x1

				// always transition on positive clock edge
				currentValue = -1 * currentValue

				c1, c2 := clock.samples()
				for i := 0; i < c1; i++ {
					outCh <- stream.Sample(currentValue)
				}

				// transition again on ones
				if bitValue != 0 {
					currentValue = -1 * currentValue
				}

				for i := 0; i < c2; i++ {
					outCh <- stream.Sample(currentValue)
				}
			}

		}
		close(outCh)
	}()

	return outCh
}
