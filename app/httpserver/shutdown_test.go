package httpserver

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestShutdownTimeoutResolution(t *testing.T) {
	cases := []struct {
		conf string
		want time.Duration
	}{
		{"", 30 * time.Second},                             // absent → default
		{"http.shutdownTimeoutMs=5000\n", 5 * time.Second}, // explicit
		{"http.shutdownTimeoutMs=0\n", 30 * time.Second},   // non-positive → default
		{"http.shutdownTimeoutMs=-1\n", 30 * time.Second},  // non-positive → default
		{"http.shutdownTimeoutMs=abc\n", 30 * time.Second}, // unparseable → default
	}
	for _, tc := range cases {
		cfg, err := ParseConfig([]byte(tc.conf), "")
		if err != nil {
			t.Fatalf("ParseConfig(%q): %v", tc.conf, err)
		}
		if got := ShutdownTimeout(cfg); got != tc.want {
			t.Errorf("ShutdownTimeout(%q) = %v, want %v", tc.conf, got, tc.want)
		}
	}
}

func TestServerSecondSignalDoesNotPanic(t *testing.T) {
	addr := freePort(t)
	handler := http.NewServeMux()
	srv := NewServer(addr, handler, 5*time.Second)

	signals := make(chan os.Signal, 2)
	done := make(chan error, 1)
	go func() { done <- srv.Run(signals, nil) }()
	waitForListen(t, addr)

	signals <- syscall.SIGTERM
	signals <- syscall.SIGTERM // second signal during shutdown must be ignored
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}
