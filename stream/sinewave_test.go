package stream

import (
	"context"
	"math"
	"testing"
)

func TestSinewave(t *testing.T) {
	cases := []struct {
		Name           string
		Amplitude      float64
		Frequency      float64
		Phase          float64
		SampleRate     int
		ExpectedValues []Sample
	}{
		{
			Name:           "1Hz-NoOffset",
			Amplitude:      1,
			Frequency:      1,
			Phase:          0,
			SampleRate:     4,
			ExpectedValues: []Sample{0, Sample(math.MaxInt32), 0, -1 * Sample(math.MaxInt32), 0},
		},
		{
			Name:           "2Hz-NoOffset",
			Amplitude:      1,
			Frequency:      2,
			Phase:          0,
			SampleRate:     4,
			ExpectedValues: []Sample{0, 0, 0, 0, 0},
		},
		{
			Name:           "2Hz-180DegreeOffset",
			Amplitude:      1,
			Frequency:      2,
			Phase:          180,
			SampleRate:     4,
			ExpectedValues: []Sample{0, 0, 0, 0, 0},
		},
		{
			Name:           "2Hz-90DegreeOffset",
			Amplitude:      1,
			Frequency:      2,
			Phase:          90,
			SampleRate:     4,
			ExpectedValues: []Sample{Sample(math.MaxInt32), -1 * Sample(math.MaxInt32), Sample(math.MaxInt32), -1 * Sample(math.MaxInt32)},
		},
		{
			Name:           "2Hz-90DegreeOffset-HalfAmplitude",
			Amplitude:      0.5,
			Frequency:      2,
			Phase:          90,
			SampleRate:     4,
			ExpectedValues: []Sample{1073741823, -1073741823, 1073741823, -1073741823},
		},
		{
			Name:           "2Hz-90DegreeOffset-HalfAmplitude-8sps",
			Amplitude:      0.5,
			Frequency:      2,
			Phase:          90,
			SampleRate:     8,
			ExpectedValues: []Sample{1073741823, 0, -1073741823, 0, 1073741823, 0, -1073741823, 0},
		},
		{
			Name:           "10Hz-100sps",
			Amplitude:      0.5,
			Frequency:      10,
			Phase:          0,
			SampleRate:     100,
			ExpectedValues: []Sample{0, 631129608, 1021189158, 1021189158, 631129608, 0, -631129608, -1021189158, -1021189158, -631129608, 0},
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(st *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			s := Sinewave(ctx, 0, c.Amplitude, c.Frequency, c.Phase, c.SampleRate)
			for i, expected := range c.ExpectedValues {
				v, ok := <-s
				if !ok {
					st.Fatal("Channel closed unexpectedly")
				}
				if v != expected {
					st.Errorf("Got %d at sample %d, expected %d", v, i, expected)
				}
			}
			cancel()
			// get one more value since cancel doesn't guarantee that we don't have an unread value in the channel
			_, ok := <-s
			_, ok = <-s
			if ok {
				st.Errorf("Channel not closed properly")
			}

		})
	}
}
