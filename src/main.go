package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
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
func config_init(logPath string, time_window int, output string) Config {
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
		Level:     slog.LevelDebug,
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
		device = "en1" // this is my default interface on this machine (MacOS)
		filter = "tcp port 80"
	)

	packets, handle, err := pkt_capture(device, filter)
	if err != nil {
		return err
	}
	defer handle.Close()

	count := 0
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				config.logger.Info(
					"All the time has passed",
					"time (seconds)", config.timeWindow,
					"packets", count,
				)
				return nil
			}
			config.logger.Info("Process Interrumped", "packets", count)
			return ctx.Err()
		case pkt, ok := <-packets:
			if !ok {
				config.logger.Info("Closed source", "packets", count)
				return nil
			}
			_ = pkt // Nothing to do here
			count += 1
			config.logger.Info(
				"just running brother",
				"packtes", count,
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
	config := config_init(*logPath, *seconds, "stdout")

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
