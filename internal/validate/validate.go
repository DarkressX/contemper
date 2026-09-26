// Package validate checks the merged rootfs against contemper's fixed
// path contract: a kernel, an initrd and a non-empty command line at a
// fixed location, and an init binary.
package validate

import (
	"fmt"
	"strings"

	"github.com/contemper-project/contemper/internal/rootfs"
)

// Fixed paths every contemper target relies on.
const (
	KernelPath    = "/boot/contemper/vmlinuz"
	InitrdPath    = "/boot/contemper/initrd"
	CmdlinePath   = "/boot/contemper/cmdline"
	InitPath      = "/sbin/init"
	OSReleasePath = "/etc/os-release"
)

// Result carries the file content read while validating the contract, so
// callers (the UKI builder) do not need to re-read the rootfs.
type Result struct {
	Kernel    []byte
	Initrd    []byte
	Cmdline   string
	OSRelease []byte // nil if the image has none
}

// Validate checks rfs against the fixed-path contract and returns the
// file contents needed to build the UKI.
func Validate(rfs *rootfs.Rootfs) (*Result, error) {
	kernel, err := rfs.ReadFile(KernelPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: %w", err)
	}
	initrd, err := rfs.ReadFile(InitrdPath)
	if err != nil {
		return nil, fmt.Errorf("initrd: %w", err)
	}
	cmdlineRaw, err := rfs.ReadFile(CmdlinePath)
	if err != nil {
		return nil, fmt.Errorf("cmdline: %w", err)
	}
	cmdline := strings.TrimSpace(string(cmdlineRaw))
	if cmdline == "" {
		return nil, fmt.Errorf("cmdline: %s is empty", CmdlinePath)
	}

	if _, err := rfs.Resolve(InitPath); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}

	var osRelease []byte
	if _, ok := rfs.Lookup(OSReleasePath); ok {
		osRelease, err = rfs.ReadFile(OSReleasePath)
		if err != nil {
			return nil, fmt.Errorf("os-release: %w", err)
		}
	}

	return &Result{Kernel: kernel, Initrd: initrd, Cmdline: cmdline, OSRelease: osRelease}, nil
}
