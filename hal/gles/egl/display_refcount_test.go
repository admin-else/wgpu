// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package egl

import "testing"

// withFakeTerminate swaps terminateDisplayFn for the duration of fn and
// records which displays were actually terminated.
func withFakeTerminate(t *testing.T, fn func(terminated *[]EGLDisplay)) {
	t.Helper()
	displayRefsMu.Lock()
	displayRefs = map[EGLDisplay]int{}
	displayRefsMu.Unlock()

	prev := terminateDisplayFn
	var terminated []EGLDisplay
	terminateDisplayFn = func(dpy EGLDisplay) EGLBoolean {
		terminated = append(terminated, dpy)
		return True
	}
	t.Cleanup(func() {
		terminateDisplayFn = prev
		displayRefsMu.Lock()
		displayRefs = map[EGLDisplay]int{}
		displayRefsMu.Unlock()
	})

	fn(&terminated)
}

func TestSharedDisplayTerminatedOnceOnLastRelease(t *testing.T) {
	const dpy EGLDisplay = 0xdead_0001

	withFakeTerminate(t, func(terminated *[]EGLDisplay) {
		retainDisplay(dpy)
		retainDisplay(dpy)

		// First release must NOT terminate: another Context still holds it.
		releaseDisplay(dpy)
		if len(*terminated) != 0 {
			t.Fatalf("display terminated too early: %v", *terminated)
		}

		// Last release terminates exactly once.
		releaseDisplay(dpy)
		if len(*terminated) != 1 || (*terminated)[0] != dpy {
			t.Fatalf("want exactly one terminate of %#x, got %v", dpy, *terminated)
		}

		// Refcount entry must be gone.
		displayRefsMu.Lock()
		_, ok := displayRefs[dpy]
		displayRefsMu.Unlock()
		if ok {
			t.Fatal("display refcount entry leaked")
		}
	})
}

func TestSingleDisplayTerminatedOnFirstRelease(t *testing.T) {
	const dpy EGLDisplay = 0xdead_0002

	withFakeTerminate(t, func(terminated *[]EGLDisplay) {
		retainDisplay(dpy)
		releaseDisplay(dpy)
		if len(*terminated) != 1 || (*terminated)[0] != dpy {
			t.Fatalf("want exactly one terminate of %#x, got %v", dpy, *terminated)
		}
	})
}

func TestNoDisplayIsIgnored(t *testing.T) {
	withFakeTerminate(t, func(terminated *[]EGLDisplay) {
		retainDisplay(NoDisplay)
		releaseDisplay(NoDisplay)
		if len(*terminated) != 0 {
			t.Fatalf("NoDisplay must never be terminated, got %v", *terminated)
		}
		displayRefsMu.Lock()
		n := len(displayRefs)
		displayRefsMu.Unlock()
		if n != 0 {
			t.Fatalf("NoDisplay must not be tracked, map has %d entries", n)
		}
	})
}
