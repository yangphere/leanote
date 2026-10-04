package httpserver

import (
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func portFree(addr string) bool {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func waitForListen(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server never listened on %s", addr)
}

func TestServerServesAndShutsDownGracefully(t *testing.T) {
	addr := freePort(t)
	handler := http.NewServeMux()
	handler.HandleFunc("/fast", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	srv := NewServer(addr, handler, 5*time.Second)

	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- srv.Run(signals, nil) }()

	waitForListen(t, addr)

	resp, err := http.Get("http://" + addr + "/fast")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	signals <- syscall.SIGTERM
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after SIGTERM")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if portFree(addr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("port not released after shutdown")
}

func TestServerShutdownWaitsForInFlightRequest(t *testing.T) {
	addr := freePort(t)
	release := make(chan struct{})
	handler := http.NewServeMux()
	handler.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	})
	srv := NewServer(addr, handler, 5*time.Second)

	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- srv.Run(signals, nil) }()
	waitForListen(t, addr)

	inFlight := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			inFlight <- err
			return
		}
		resp.Body.Close()
		inFlight <- nil
	}()

	time.Sleep(100 * time.Millisecond) // let the request reach the handler
	signals <- syscall.SIGTERM
	time.Sleep(100 * time.Millisecond) // let Shutdown enter its waiting phase
	close(release)                     // finish the in-flight request

	select {
	case err := <-inFlight:
		if err != nil {
			t.Fatalf("in-flight request failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight request not completed within shutdown bound")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestServerShutdownTimesOutWithNonNilError(t *testing.T) {
	addr := freePort(t)
	handler := http.NewServeMux()
	handler.HandleFunc("/stuck", func(w http.ResponseWriter, r *http.Request) {
		select {} // never returns; simulate a hung request
	})
	srv := NewServer(addr, handler, 150*time.Millisecond)

	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- srv.Run(signals, nil) }()
	waitForListen(t, addr)

	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + addr + "/stuck")
		if err == nil {
			resp.Body.Close()
		}
	}()
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	signals <- syscall.SIGTERM
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("timed-out shutdown must return a non-nil error")
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Fatalf("shutdown returned after %v, want ~bound (150ms)", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after shutdown timeout")
	}
}
