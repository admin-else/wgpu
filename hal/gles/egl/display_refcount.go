// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package egl

import "sync"

// EGL display handles are process-global state: the EGL spec says that
// multiple calls to eglGetPlatformDisplay (or eglGetDisplay) with the same
// arguments return the SAME EGLDisplay handle, and libglvnd/Mesa do exactly
// that. Several egl.Context values can therefore share one display.
//
// eglTerminate destroys the whole display, so terminating it while another
// Context still references it frees driver state out from under that Context.
// The next EGL call, or a second eglTerminate, then dereferences the freed
// display and segfaults (observed as SIGSEGV in eglTerminate on Mesa/AMD).
//
// Track how many Contexts reference each display and only call eglTerminate
// when the last reference is dropped. Mirrors wgpu-hal's
// DISPLAYS_REFERENCE_COUNT (gfx-rs/wgpu #5349, fixed by PR #5351).
var (
	displayRefsMu sync.Mutex
	displayRefs   = map[EGLDisplay]int{}

	// terminateDisplayFn is indirected so tests can observe termination
	// without touching a real EGL driver.
	terminateDisplayFn = Terminate
)

// retainDisplay records that a Context now references dpy. Call it once per
// successfully initialized display.
func retainDisplay(dpy EGLDisplay) {
	if dpy == NoDisplay {
		return
	}
	displayRefsMu.Lock()
	displayRefs[dpy]++
	displayRefsMu.Unlock()
}

// releaseDisplay drops one reference to dpy and terminates the display only
// when no Context references it anymore.
func releaseDisplay(dpy EGLDisplay) {
	if dpy == NoDisplay {
		return
	}
	displayRefsMu.Lock()
	count := displayRefs[dpy]
	if count > 1 {
		displayRefs[dpy] = count - 1
		displayRefsMu.Unlock()
		return
	}
	delete(displayRefs, dpy)
	displayRefsMu.Unlock()
	terminateDisplayFn(dpy)
}
