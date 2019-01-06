package stream

import (
	"context"
	"reflect"
)

func MergeChannels(ctx context.Context, bufferLen int, inCh ...chan Sample) chan []Sample {
	mergedCh := make(chan []Sample, bufferLen)
	go func() {
		for {
			v := make([]Sample, len(inCh))
			selectMap := make([]int, len(inCh))
			cases := make([]reflect.SelectCase, len(inCh)+1)
			cases[0] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}
			for i, ch := range inCh {
				cases[i+1] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
				selectMap[i] = i
			}

			for len(cases) > 1 {
				// create select cases, starting with one for context completion channel
				chosen, value, ok := reflect.Select(cases)
				if chosen == 0 || !ok {
					// Done
					close(mergedCh)
					return
				}

				chIdx := selectMap[chosen-1]
				v[chIdx] = value.Interface().(Sample)
				cases = append(cases[:chosen], cases[chosen+1:]...)
				selectMap = append(selectMap[:chosen-1], selectMap[chosen:]...)
			}
			mergedCh <- v
		}

	}()
	return mergedCh
}
