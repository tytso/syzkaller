// Copyright 2015 syzkaller project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package prog

import (
	"runtime"
	"strings"
	"testing"
)

func TestResourceCtors(t *testing.T) {
	target, err := GetTarget("linux", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range target.Syscalls {
		for _, res := range c.inputResources() {
			if len(target.calcResourceCtors(res.Desc.Kind, true)) == 0 {
				t.Errorf("call %v requires input resource %v, but there are no calls that can create this resource", c.Name, res.Desc.Name)
			}
		}
	}
}

func TestTransitivelyEnabledCalls(t *testing.T) {
	t.Parallel()
	target, err := GetTarget("linux", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	calls := make(map[*Syscall]bool)
	for _, c := range target.Syscalls {
		calls[c] = true
	}
	if trans, disabled := target.TransitivelyEnabledCalls(calls); len(disabled) != 0 {
		for c, reason := range disabled {
			t.Logf("disabled %v: %v", c.Name, reason)
		}
		t.Fatalf("can't create some resource")
	} else if len(trans) != len(calls) {
		t.Fatalf("transitive syscalls are not full")
	} else {
		for c, ok := range trans {
			if !ok {
				t.Fatalf("syscalls %v is false in transitive map", c.Name)
			}
		}
	}
	delete(calls, target.SyscallMap["epoll_create"])
	if trans, disabled := target.TransitivelyEnabledCalls(calls); len(disabled) != 0 || len(trans) != len(calls) {
		t.Fatalf("still must be able to create epoll fd with epoll_create1")
	}
	delete(calls, target.SyscallMap["epoll_create1"])
	trans, disabled := target.TransitivelyEnabledCalls(calls)
	if len(calls)-6 != len(trans) ||
		trans[target.SyscallMap["epoll_ctl$EPOLL_CTL_ADD"]] ||
		trans[target.SyscallMap["epoll_ctl$EPOLL_CTL_MOD"]] ||
		trans[target.SyscallMap["epoll_ctl$EPOLL_CTL_DEL"]] ||
		trans[target.SyscallMap["epoll_wait"]] ||
		trans[target.SyscallMap["epoll_pwait"]] ||
		trans[target.SyscallMap["kcmp$KCMP_EPOLL_TFD"]] {
		t.Fatalf("epoll fd is not disabled")
	}
	if len(disabled) != 6 {
		t.Fatalf("disabled %v syscalls, want 6", len(disabled))
	}
	for c, reason := range disabled {
		if !strings.Contains(reason, "no syscalls can create resource fd_epoll, enable some syscalls that can create it [epoll_create epoll_create1]") {
			t.Fatalf("%v: wrong disable reason: %v", c.Name, reason)
		}
	}
}

func TestClockGettime(t *testing.T) {
	t.Parallel()
	target, err := GetTarget("linux", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	calls := make(map[*Syscall]bool)
	for _, c := range target.Syscalls {
		calls[c] = true
	}
	// Removal of clock_gettime should disable all calls that accept timespec/timeval.
	delete(calls, target.SyscallMap["clock_gettime"])
	trans, disabled := target.TransitivelyEnabledCalls(calls)
	if len(trans)+10 > len(calls) || len(trans)+len(disabled) != len(calls) || len(trans) == 0 {
		t.Fatalf("clock_gettime did not disable enough calls: before %v, after %v, disabled %v",
			len(calls), len(trans), len(disabled))
	}
}
