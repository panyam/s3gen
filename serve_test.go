package s3gen

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/radovskyb/watcher"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func get(url string) (int, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), err
}

func TestServeStopsOnSIGINT(t *testing.T) {
	// Without this, a Serve that ignores SIGINT would let the signal kill the test binary.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGINT)
	defer signal.Stop(guard)

	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &Site{OutputDir: out, PathPrefix: "/docs"}
	addr := freeAddr(t)
	done := make(chan error, 1)
	go func() { done <- site.Serve(addr) }()

	url := "http://" + addr + "/docs/hello.txt"
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, body, err := get(url)
		if err == nil {
			if code != http.StatusOK || body != "hello" {
				t.Fatalf("GET %s = %d %q, want 200 \"hello\"", url, code, body)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if code, _, _ := get("http://" + addr + "/hello.txt"); code != http.StatusNotFound {
		t.Fatalf("GET outside PathPrefix = %d, want 404", code)
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after SIGINT, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return within 5s of SIGINT")
	}
	if _, _, err := get(url); err == nil {
		t.Fatal("still serving after Serve returned")
	}
}

func TestStopWatchingWithoutWatch(t *testing.T) {
	(&Site{}).StopWatching()
}

func TestStopWatchingDropsTheWatcher(t *testing.T) {
	site := &Site{reloadWatcher: watcher.New()}
	site.StopWatching()
	if site.reloadWatcher != nil {
		t.Fatal("reloadWatcher is still set after StopWatching")
	}
}
