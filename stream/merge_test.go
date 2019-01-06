package stream

import (
	"context"
	"testing"
)

func TestMergeCh(t *testing.T) {
	ch1 := make(chan Sample)
	ch2 := make(chan Sample)

	outCh := MergeChannels(context.Background(), 0, ch1, ch2)

	v1 := []Sample{0, 1, 2, 3, 4, 5}
	v2 := []Sample{5, 4, 3, 2, 1}

	feedCh := func(ch chan Sample, values []Sample) {
		for _, v := range values {
			ch <- v
		}
		close(ch)
	}

	go feedCh(ch1, v1)
	go feedCh(ch2, v2)

	for i := 0; i < 5; i++ {
		if r, ok := <-outCh; !ok || r[0] != v1[i] || r[1] != v2[i] {
			t.Errorf("Got wrong value: %v, expected %v", r, []Sample{v1[i], v2[i]})
		}
	}
	r, ok := <-outCh
	if ok {
		t.Errorf("Got value from channel that should have been closed: %v", r)
	}
}
