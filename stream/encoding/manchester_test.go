package encoding

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/azenk/audio/stream"
	"github.com/go-test/deep"
)

func TestSampleClock(t *testing.T) {
	cases := []struct {
		Name       string
		Frequency  float64
		SampleRate int
	}{
		{
			Name:       "4Hz",
			Frequency:  4,
			SampleRate: 44100,
		},
		{
			Name:       "100Hz",
			Frequency:  100,
			SampleRate: 44100,
		},
		{
			Name:       "2400Hz",
			Frequency:  2400,
			SampleRate: 44100,
		},
		{
			Name:       "4413Hz",
			Frequency:  4413,
			SampleRate: 44100,
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(st *testing.T) {
			clock := newSampleClock(c.Frequency, c.SampleRate)

			testInterval := 10
			var count int
			for i := 0; i < int(c.Frequency)*testInterval; i++ {
				c1, c2 := clock.samples()
				count += c1 + c2
			}

			if count != c.SampleRate*testInterval && count != c.SampleRate*testInterval-1 {
				st.Errorf("Got fewer samples than expected %d, expected either %d or %d", count, c.SampleRate*testInterval-1, c.SampleRate*testInterval)
			}

		})
	}
}

func TestDifferentialManchester(t *testing.T) {
	cases := []struct {
		Name            string
		InputBytes      []byte
		BitsPerSecond   float64
		Amplitude       float64
		SampleRate      int
		ExpectedSamples []stream.Sample
	}{
		{
			Name:          "Zero",
			InputBytes:    []byte{0x00},
			BitsPerSecond: 5,
			Amplitude:     1.0,
			SampleRate:    10,
			ExpectedSamples: []stream.Sample{
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
			},
		},
		{
			Name:          "A5",
			InputBytes:    []byte{0xA5},
			BitsPerSecond: 5,
			Amplitude:     1.0,
			SampleRate:    10,
			ExpectedSamples: []stream.Sample{
				-1 * 0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, -1 * 0x7FFFFFFF,
				0x7FFFFFFF, 0x7FFFFFFF,
				-1 * 0x7FFFFFFF, 0x7FFFFFFF,
			},
		},
		{
			Name:          "A5A5",
			InputBytes:    []byte{0xA5, 0xA5},
			BitsPerSecond: 5,
			Amplitude:     1.0 / math.MaxInt32,
			SampleRate:    10,
			ExpectedSamples: []stream.Sample{
				-1, 1,
				-1, -1,
				1, -1,
				1, 1,
				-1, -1,
				1, -1,
				1, 1,
				-1, 1, // end of first byte
				-1, 1,
				-1, -1,
				1, -1,
				1, 1,
				-1, -1,
				1, -1,
				1, 1,
				-1, 1,
			},
		},
		{
			Name:          "A5-20",
			InputBytes:    []byte{0xA5},
			BitsPerSecond: 5,
			Amplitude:     1.0 / math.MaxInt32,
			SampleRate:    20,
			ExpectedSamples: []stream.Sample{
				-1, -1, 1, 1,
				-1, -1, -1, -1,
				1, 1, -1, -1,
				1, 1, 1, 1,
				-1, -1, -1, -1,
				1, 1, -1, -1,
				1, 1, 1, 1,
				-1, -1, 1, 1,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(st *testing.T) {
			inCh := make(chan byte, len(c.InputBytes))
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(3*time.Second))
			defer cancel()
			outCh := DifferentialManchester(ctx, 1, c.BitsPerSecond, c.Amplitude, c.SampleRate, inCh)

			for _, b := range c.InputBytes {
				inCh <- b
			}
			close(inCh)

			outSamples := make([]stream.Sample, 0, len(c.InputBytes)*16)
			for sample := range outCh {
				outSamples = append(outSamples, sample)
			}

			if diff := deep.Equal(outSamples, c.ExpectedSamples); len(diff) > 0 {
				st.Error("Output samples not equal to expected:")
				for _, l := range diff {
					st.Error(l)
				}
			}

		})
	}
}
