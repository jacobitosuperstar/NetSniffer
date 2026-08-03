package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/tcpassembly"
)

// Config stores the logging configuration and the time window on which the
// software will run
type Config struct {
	logger     *slog.Logger
	timeWindow int
}

// config_init Initializes the configuration object returning a Config struct
// that contains the timeWindow of operation and the logger. This panics if
// we cannot open the file on which we are going to write the logs.
func NewConfig(logPath string, time_window int, output string) Config {
	// log file
	var logOut io.Writer
	switch output {
	case "stdout":
		logOut = os.Stdout
	default:
		file, err := os.OpenFile(
			logPath,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644,
		)
		if err != nil {
			panic(fmt.Errorf("Failed to open, create or write in %s file", logPath))
		}
		logOut = file
	}
	// options for the logger
	options := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}
	handler := slog.NewTextHandler(logOut, options)
	log := slog.New(handler)
	config := Config{log, time_window}
	return config
}

// worker This is where the code runs
func worker(ctx context.Context, config *Config) error {
	const (
		device    = "en1" // this is my default interface on this machine (MacOS)
		filter    = "tcp port 80"
		idleTimer = 1 // minutes
	)

	packets, handle, err := pkt_capture(device, filter)
	if err != nil {
		return err
	}
	defer handle.Close()

	// start the count!!!!!!!
	counter := NewCounter()
	var wg sync.WaitGroup
	asf := &AssemblerStreamFactory{
		counter: counter,
		wg:      &wg,
	}
	pool := tcpassembly.NewStreamPool(asf)
	assembler := tcpassembly.NewAssembler(pool)

	// We will check which connections are idle for too lond and send them to
	// the reader. A slow connection cannot hang forever.
	flush := time.NewTicker(time.Minute)
	defer flush.Stop()

	for {
		select {
		case <-ctx.Done():
			// send everything to the reader and wait for them to be processed
			// if possible.
			assembler.FlushAll()
			wg.Wait()

			if ctx.Err() == context.DeadlineExceeded {
				config.logger.Info(
					"All the time has passed",
					"time (seconds)", config.timeWindow,
					"traffic", counter.total,
				)
				// FINAL REPORT INTO THE CONSOLE
				counter.PrettyRanking(os.Stdout)
				return nil
			}
			config.logger.Error("Process Interrumped", "traffic", counter.total)
			// FINAL REPORT INTO THE CONSOLE
			counter.PrettyRanking(os.Stdout)
			return ctx.Err()
		case <-flush.C:
			// if the connection is alive for more than X time, send it to the
			// reader already.
			assembler.FlushOlderThan(time.Now().Add(-(idleTimer) * time.Minute))
		case pkt, ok := <-packets:
			if !ok {
				assembler.FlushAll()
				wg.Wait()
				config.logger.Info("Closed source", "traffic", counter.total)
				return nil
			}
			netLayer := pkt.NetworkLayer()
			transportLayer := pkt.TransportLayer()

			// NOTE: Malformed packets can still pass the BPF offsets and
			// validations as they don't check for content.
			if netLayer == nil || transportLayer == nil {
				config.logger.Warn("Malformed packet was here")
				continue
			}

			// If this is a performance issue we can nuke this and just hope
			// for the BPF to always work correctly.
			tcp, ok := transportLayer.(*layers.TCP)
			if !ok {
				config.logger.Error("The BPF is not working correctly as only TCP connections should be here")
				continue
			}

			// HANDOFF
			assembler.AssembleWithTimestamp(
				netLayer.NetworkFlow(),
				tcp,
				pkt.Metadata().Timestamp,
			)

			total, hosts := counter.View()
			config.logger.Debug(
				"just running brother",
				"total traffic", total,
				"hosts", hosts,
			)
		}
	}
}

// run parses the args and calls the worker when we are OK
func run(argv []string) error {
	fs := flag.NewFlagSet("main", flag.ContinueOnError)
	logPath := fs.String("log", "", "path to the log file")
	seconds := fs.Int("seconds", 0, "how long the program runs (seconds)")

	if err := fs.Parse(argv); err != nil {
		return err
	}

	if *logPath == "" {
		return fmt.Errorf("log path is required")
	}

	if *seconds <= 0 {
		return fmt.Errorf("seconds must be possitive, got %d\n", *seconds)
	}
	duration := time.Duration(*seconds) * time.Second

	// for development we will spit everything to stdout
	config := NewConfig(*logPath, *seconds, "stdout")

	// OS signals
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// adding the timeout
	ctx, stop = context.WithTimeout(ctx, duration)
	defer stop()

	if err := worker(ctx, &config); err != nil {
		config.logger.Error("Error running the worker", "error", err)
		return err
	}
	config.logger.Info("Finished")
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
