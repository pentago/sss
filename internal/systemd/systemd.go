// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package systemd implements the sd_notify protocol for Type=notify services.
package systemd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// sd_notify states, see https://www.freedesktop.org/software/systemd/man/sd_notify.html.
const (
	// SdNotifyReady tells the service manager that startup is finished.
	SdNotifyReady = "READY=1"
	// SdNotifyStopping tells the service manager that the service is shutting down.
	SdNotifyStopping = "STOPPING=1"
	// SdNotifyWatchdog updates the service manager's watchdog timestamp.
	SdNotifyWatchdog = "WATCHDOG=1"
)

// HasNotifySocket reports whether the process is supervised by systemd with a notify socket.
func HasNotifySocket() bool {
	return os.Getenv("NOTIFY_SOCKET") != ""
}

// HasSystemdWatchdog reports whether the unit file configures a watchdog.
func HasSystemdWatchdog() bool {
	return os.Getenv("WATCHDOG_USEC") != ""
}

// WatchdogInterval returns the watchdog interval configured in the unit file.
func WatchdogInterval() (time.Duration, error) {
	s, err := strconv.Atoi(os.Getenv("WATCHDOG_USEC"))
	if err != nil {
		return 0, fmt.Errorf("systemd: error converting WATCHDOG_USEC: %w", err)
	}
	if s <= 0 {
		return 0, errors.New("systemd: WATCHDOG_USEC must be a positive number")
	}
	return time.Duration(s) * time.Microsecond, nil
}

// SdNotify sends state to the NOTIFY_SOCKET datagram socket; it is a no-op when that is unset.
func SdNotify(state string) error {
	addr := &net.UnixAddr{Net: "unixgram", Name: os.Getenv("NOTIFY_SOCKET")}
	if addr.Name == "" {
		return nil
	}

	conn, err := net.DialUnix(addr.Net, nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}
