package telemetry

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"runtime"
	"time"

	segment "github.com/segmentio/analytics-go/v3"
)

// Injected at build time
var segmentKey = ""

var opts struct {
	Disabled bool
	Identity string

	client segment.Client
}

// Init initialises the telemetry
func Init(disable bool, identity string) {
	opts.Disabled = disable
	if disable {
		return
	}

	if identity == "" {
		letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
		b := make([]rune, 32)
		for i := range b {
			b[i] = letters[rand.Intn(len(letters))]
		}
		identity = string(b)
	}
	opts.Identity = identity

	if segmentKey != "" {
		opts.client = segment.New(segmentKey)
	}
}

func Close() {
	if opts.client != nil {
		opts.client.Close()
	}
}

// Identity returns the identity
func Identity() string {
	return opts.Identity
}

// Enabled returns true if the telemetry is enabled
func Enabled() bool {
	return !opts.Disabled && opts.Identity != "" && opts.client != nil
}

func track(event string, props segment.Properties) {
	if !Enabled() {
		return
	}

	opts.client.Enqueue(segment.Track{
		AnonymousId: opts.Identity,
		Event:       event,
		Timestamp:   time.Now(),
		Properties:  props,
	})
}

// RecordWorkspaceStarted sends telemetry when a workspace is started
func RecordWorkspaceStarted(remoteURI string, containerRuntime string) {
	uriHash := sha256.New()
	_, _ = uriHash.Write([]byte(remoteURI))

	track("rungp_start_workspace", segment.NewProperties().
		Set("runtime", containerRuntime).
		Set("remoteURIHash", fmt.Sprintf("sha256:%x", uriHash.Sum(nil))).
		Set("GOOS", runtime.GOOS).
		Set("GOARCH", runtime.GOARCH),
	)
}
