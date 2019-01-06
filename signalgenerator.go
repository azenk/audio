package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"

	"github.com/azenk/audio/stream"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

func main() {
	cfgFile := viper.New()
	cfgFile.SetDefault("left.frequency", 1000)
	cfgFile.SetDefault("left.amplitude", 1)
	cfgFile.SetDefault("left.phase", 0)
	cfgFile.SetDefault("right.frequency", 1000)
	cfgFile.SetDefault("right.amplitude", 1)
	cfgFile.SetDefault("right.phase", 0)
	cfgFile.AddConfigPath(".")
	cfgFile.SetConfigName("signals")
	cfgFile.ReadInConfig()
	flag.Parse()

	if cpuProfileFile := cfgFile.GetString("cpuprofile"); cpuProfileFile != "" {
		f, err := os.Create(cpuProfileFile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := stream.OpenDefaultDevice(ctx, &stream.Configuration{Channels: 2})
	if err != nil {
		panic(err)
	}
	glog.Infof("Opened card with config: %s", d.Config())

	streamCh := d.Stream()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	signalsCtx, signalsCancel := context.WithCancel(context.Background())
	defer signalsCancel()

	left := stream.Sinewave(signalsCtx,
		d.Config().SampleRate()/100,
		cfgFile.GetFloat64("left.amplitude"),
		cfgFile.GetFloat64("left.frequency"),
		cfgFile.GetFloat64("left.phase"),
		d.Config().SampleRate())

	right := stream.Sinewave(signalsCtx,
		d.Config().SampleRate()/100,
		cfgFile.GetFloat64("right.amplitude"),
		cfgFile.GetFloat64("right.frequency"),
		cfgFile.GetFloat64("right.phase"),
		d.Config().SampleRate())

	lrMerged := stream.MergeChannels(signalsCtx, d.Config().SampleRate()/100, left, right)

	glog.Info("Stream started")
	var streamClosed bool
	for {
		select {
		case samples, more := <-lrMerged:
			if more {
				streamCh <- samples
			} else if !streamClosed {
				glog.Infof("No more samples from generators, closing stream channel")
				close(streamCh)
				streamClosed = true
			}
		case <-c:
			glog.Info("Got interrupt")
			signalsCancel()
		case err, more := <-d.Done():
			if err != nil {
				glog.Infof("Error streaming data: %v", err)
			}

			if !more {
				glog.Info("Exiting")
				return
			}
		}
	}

}
