//go:build native && windows

package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestAppInstanceRejectsDuplicateAndAllowsRestart(t *testing.T) {
	name := fmt.Sprintf(`Local\MabiDPSTest.%d.%d`, os.Getpid(), time.Now().UnixNano())
	first, err := openAppInstance(name)
	if err != nil {
		t.Fatal(err)
	}
	second, secondErr := openAppInstance(name)
	_ = windows.CloseHandle(first)
	if second != 0 || secondErr != windows.ERROR_ALREADY_EXISTS {
		t.Fatalf("duplicate instance: handle=%v error=%v", second, secondErr)
	}
	restarted, err := openAppInstance(name)
	if err != nil {
		t.Fatal(err)
	}
	_ = windows.CloseHandle(restarted)
}
