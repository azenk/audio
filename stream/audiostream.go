package stream

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	"github.com/yobert/alsa"
)

type Sample int32

func (s Sample) Encode(buf io.Writer, format alsa.FormatType) {
	switch format {
	case alsa.S16_LE:
		binary.Write(buf, binary.LittleEndian, int16(s>>16))
	case alsa.S32_LE:
		binary.Write(buf, binary.LittleEndian, s)
	case alsa.S16_BE:
		binary.Write(buf, binary.BigEndian, int16(s>>16))
	case alsa.S32_BE:
		binary.Write(buf, binary.BigEndian, s)
	}
}

type Configuration struct {
	PeriodSize int
	BufferSize int
	format     alsa.FormatType
	Rate       int
	Channels   int
}

func (c Configuration) SampleSizeBytes() int {
	switch c.format {
	case alsa.S16_LE:
		return 2
	case alsa.S32_LE:
		return 4
	case alsa.S16_BE:
		return 2
	case alsa.S32_BE:
		return 4
	}
	return 0
}

func (c Configuration) ChannelCount() int {
	if c.Channels == 0 {
		return 1
	}
	return c.Channels
}

func (c Configuration) SampleRate() int {
	if c.Rate == 0 {
		return 44100
	}
	return c.Rate
}

func (c Configuration) sampleFormat() alsa.FormatType {
	return c.format
}

func (c Configuration) String() string {
	return fmt.Sprintf("Period: %d, SampleSize: %d bits, Rate: %d HZ, Channels: %d", c.PeriodSize, c.SampleSizeBytes()*8, c.Rate, c.Channels)
}

func (c Configuration) OutputDelay() time.Duration {
	return time.Duration(c.PeriodSize) * time.Second / time.Duration(c.Rate)
}

type StreamDevice struct {
	cards       []*alsa.Card
	device      *alsa.Device
	doneCh      chan error
	config      *Configuration
	frameSentCh chan struct{}
}

func NewStreamDevice(device *alsa.Device) *StreamDevice {
	d := &StreamDevice{
		device: device,
		doneCh: make(chan error),
		config: &Configuration{},
	}
	return d
}

func OpenDefaultDevice(ctx context.Context, requestedConfig *Configuration) (*StreamDevice, error) {
	d := &StreamDevice{
		doneCh:      make(chan error),
		config:      &Configuration{},
		frameSentCh: make(chan struct{}),
	}

	cards, err := alsa.OpenCards()
	if err != nil {
		return nil, fmt.Errorf("Unable to open cards: %v", err)
	}

	if len(cards) == 0 {
		return nil, fmt.Errorf("Unable to get alsa device")
	}

	d.cards = cards
	card := cards[0]

	devices, err := card.Devices()
	if err != nil {
		return nil, fmt.Errorf("Unable to list devices: %v", err)
	}

	for _, dev := range devices {
		if dev.Type == alsa.PCM && dev.Play {
			d.device = dev
			break
		}
	}

	config, err := d.Open(requestedConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to open device for streaming: %v", err)
	}
	d.config = config
	go d.closeWithContext(ctx)
	return d, nil
}

func (d *StreamDevice) Config() *Configuration {
	return d.config
}

func (d *StreamDevice) Open(requestedConfig *Configuration) (*Configuration, error) {
	var err error

	if err = d.device.Open(); err != nil {
		return nil, err
	}

	// Cleanup device when done or force cleanup after 3 seconds.

	channels, err := d.device.NegotiateChannels(requestedConfig.ChannelCount())
	if err != nil {
		return nil, err
	}

	rate, err := d.device.NegotiateRate(requestedConfig.SampleRate())
	if err != nil {
		return nil, err
	}

	format, err := d.device.NegotiateFormat(alsa.S16_LE, alsa.S32_LE)
	if err != nil {
		return nil, err
	}

	// A 50ms period is a sensible value to test low-ish latency.
	// We adjust the buffer so it's of minimal size (period * 2) since it appear ALSA won't
	// start playback until the buffer has been filled to a certain degree and the automatic
	// buffer size can be quite large.
	// Some devices only accept even periods while others want powers of 2.
	wantPeriodSize := 1024 // ~3ms @ 44100Hz

	periodSize, err := d.device.NegotiatePeriodSize(wantPeriodSize)
	if err != nil {
		return nil, err
	}

	bufferSize, err := d.device.NegotiateBufferSize(wantPeriodSize * 2)
	if err != nil {
		return nil, err
	}

	if err = d.device.Prepare(); err != nil {
		return nil, err
	}

	c := &Configuration{
		BufferSize: bufferSize,
		PeriodSize: periodSize,
		format:     format,
		Rate:       rate,
		Channels:   channels,
	}
	d.config = c

	return c, nil
}

func (d *StreamDevice) closeWithContext(ctx context.Context) {
	<-ctx.Done()
	d.Close()
}

func (d *StreamDevice) Close() {
	alsa.CloseCards(d.cards)
	glog.Info("Closing device")
	d.device.Close()
	glog.Info("Device Closed")
}

func (d *StreamDevice) Done() chan error {
	return d.doneCh
}

func (d *StreamDevice) encodeSamples(inputCh chan []Sample) {
	var buf bytes.Buffer
	var frameBytes int = d.config.PeriodSize * d.config.Channels * d.config.SampleSizeBytes()
	glog.Infof("Output frame size: %d bytes", frameBytes)

	frameCh := make(chan []byte, 2)
	go d.writeFrame(frameCh)

	go func() {
		for samples := range inputCh {
			for _, s := range samples {
				s.Encode(&buf, d.config.format)
			}
			if buf.Len() >= frameBytes {
				writeBuf := make([]byte, frameBytes)
				n, err := buf.Read(writeBuf)
				if err != nil {
					d.doneCh <- err
				}
				if n != frameBytes {
					glog.Infof("Writing non-standard frame: %d bytes", n)
				}
				frameCh <- writeBuf
			}

		}
		glog.Infof("Done recieving samples, closing frame channel")
		close(frameCh)
	}()
}

// FrameSent returns a channel that will be signaled whenever a frame has been sent
func (d *StreamDevice) FrameSent() chan struct{} {
	return d.frameSentCh
}

func (d *StreamDevice) writeFrame(frameCh chan []byte) {
	frameUnitSize := d.config.SampleSizeBytes() * d.config.Channels
	for frame := range frameCh {
		err := d.device.Write(frame, len(frame)/frameUnitSize)
		if err != nil {
			d.doneCh <- err
		}
		select {
		case d.frameSentCh <- struct{}{}:
		default:
		}
	}
	close(d.doneCh)
}

func (d *StreamDevice) Stream() chan []Sample {
	sampleCh := make(chan []Sample, 100)

	d.encodeSamples(sampleCh)

	return sampleCh
}
