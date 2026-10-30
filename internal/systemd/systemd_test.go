package systemd

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestSdNotify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	t.Setenv("NOTIFY_SOCKET", path)

	if err := SdNotify(SdNotifyReady); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != "READY=1" {
		t.Fatalf("got %q, %v", buf[:n], err)
	}
}

func TestSdNotifyWithoutSocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")
	if err := SdNotify(SdNotifyReady); err != nil {
		t.Fatal(err)
	}
}

func TestWatchdogInterval(t *testing.T) {
	t.Setenv("WATCHDOG_USEC", "30000000")
	if d, err := WatchdogInterval(); err != nil || d != 30*time.Second {
		t.Fatalf("got %v, %v", d, err)
	}
	for _, v := range []string{"0", "x"} {
		t.Setenv("WATCHDOG_USEC", v)
		if _, err := WatchdogInterval(); err == nil {
			t.Errorf("%q: expected error", v)
		}
	}
}
