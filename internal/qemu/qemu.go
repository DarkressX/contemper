// Package qemu implements `deploy --to local-qemu`: booting a contemper
// bundle's disk under a host-discovered qemu-system-* binary with UEFI
// firmware, snapshot=on so the bundle disk stays pristine.
package qemu

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/contemper-project/contemper/internal/hostenv"
	"github.com/contemper-project/contemper/internal/progress"
)

// archInfo carries everything about booting a given architecture that
// isn't host-dependent.
type archInfo struct {
	systemBinary     string
	machine          string
	pflashCandidates []string
	biosCandidates   []string
}

var archTable = map[string]archInfo{
	"arm64": {
		systemBinary: "qemu-system-aarch64",
		machine:      "virt",
		pflashCandidates: []string{
			"/opt/homebrew/share/qemu/edk2-aarch64-code.fd",
			"/opt/homebrew/opt/qemu/share/qemu/edk2-aarch64-code.fd",
			"/usr/local/share/qemu/edk2-aarch64-code.fd",
			"/usr/local/opt/qemu/share/qemu/edk2-aarch64-code.fd",
			"/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
			"/usr/share/AAVMF/AAVMF_CODE.fd",
		},
		biosCandidates: []string{
			"/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
			"/usr/share/AAVMF/AAVMF_CODE.fd",
		},
	},
	"amd64": {
		systemBinary: "qemu-system-x86_64",
		machine:      "q35",
		pflashCandidates: []string{
			"/opt/homebrew/share/qemu/edk2-x86_64-code.fd",
			"/opt/homebrew/opt/qemu/share/qemu/edk2-x86_64-code.fd",
			"/usr/local/share/qemu/edk2-x86_64-code.fd",
			"/usr/local/opt/qemu/share/qemu/edk2-x86_64-code.fd",
			"/usr/share/OVMF/OVMF_CODE.fd",
			"/usr/share/ovmf/OVMF.fd",
		},
		biosCandidates: []string{
			"/usr/share/OVMF/OVMF_CODE.fd",
			"/usr/share/ovmf/OVMF.fd",
		},
	},
}

func firstExisting(paths []string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// Options configures Deploy.
type Options struct {
	Arch          string
	DiskPath      string
	DiskFormat    string // "qcow2" or "raw"
	SerialLogPath string // if empty, a temp file is used
	Expect        string
	Timeout       time.Duration
	// Progress, if non-nil, receives a "booting" line, the qemu argv
	// (--verbose only), the live serial console, and a final match/error
	// line.
	Progress *progress.Reporter
}

// BuildArgs returns the qemu-system-* argv (excluding argv[0]) for opts,
// and the path firmware/disk resolution used. Split out from Deploy so
// the argument-building logic can be unit-tested without qemu installed.
func BuildArgs(opts Options) (args []string, err error) {
	info, ok := archTable[opts.Arch]
	if !ok {
		return nil, fmt.Errorf("no qemu configuration for arch %q", opts.Arch)
	}
	return buildArgs(info, opts)
}

// accelInfo picks the acceleration backend: HVF on darwin, KVM if
// /dev/kvm exists, TCG otherwise.
func accelInfo() (accel, cpu string) {
	switch {
	case runtime.GOOS == "darwin":
		return "hvf", "host"
	case fileExists("/dev/kvm"):
		return "kvm", "host"
	default:
		return "tcg", "max"
	}
}

func buildArgs(info archInfo, opts Options) (args []string, err error) {
	args = append(args, "-M", info.machine)

	accel, cpu := accelInfo()
	args = append(args, "-accel", accel, "-cpu", cpu)

	args = append(args, "-m", "1G", "-smp", "2")

	if pflash := firstExisting(info.pflashCandidates); pflash != "" {
		args = append(args, "-drive", fmt.Sprintf("if=pflash,format=raw,readonly=on,file=%s", pflash))
	} else if bios := firstExisting(info.biosCandidates); bios != "" {
		args = append(args, "-bios", bios)
	} else {
		return nil, fmt.Errorf("no UEFI firmware found for %s; install qemu (brew install qemu) or the appropriate *-efi/OVMF package", opts.Arch)
	}

	format := opts.DiskFormat
	if format == "" {
		format = "qcow2"
	}
	args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio,format=%s,snapshot=on", opts.DiskPath, format))
	args = append(args, "-netdev", "user,id=net0", "-device", "virtio-net-pci,netdev=net0")
	args = append(args, "-nographic", "-monitor", "none", "-serial", "file:"+opts.SerialLogPath)

	return args, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Deploy boots the disk described by opts under qemu. With opts.Expect
// set, it returns nil as soon as that string appears on the serial
// console (killing qemu), or an error (with the log tail) on timeout.
// Without it, Deploy waits for qemu to exit on its own.
func Deploy(opts Options) error {
	rep := opts.Progress

	info, ok := archTable[opts.Arch]
	if !ok {
		rep.Fail("deploy", fmt.Sprintf("no qemu configuration for arch %q", opts.Arch), "")
		return fmt.Errorf("no qemu configuration for arch %q", opts.Arch)
	}
	binPath, err := hostenv.Required(info.systemBinary)
	if err != nil {
		rep.Fail("deploy", err.Error(), "")
		return err
	}

	if opts.SerialLogPath == "" {
		f, err := os.CreateTemp("", "contemper-serial-*.log")
		if err != nil {
			return fmt.Errorf("creating serial log: %w", err)
		}
		opts.SerialLogPath = f.Name()
		f.Close()
	}

	args, err := BuildArgs(opts)
	if err != nil {
		rep.Fail("deploy", err.Error(), "")
		return err
	}

	accel, _ := accelInfo()
	rep.Line("🚀", "booting "+filepath.Base(opts.DiskPath), fmt.Sprintf("UEFI/%s · 1 GiB", strings.ToUpper(accel)))
	rep.VerboseCmd(binPath, args)

	cmd := exec.Command(binPath, args...)
	var toolErr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &toolErr)
	if err := cmd.Start(); err != nil {
		rep.Fail("deploy", fmt.Sprintf("starting %s failed", binPath), err.Error())
		return fmt.Errorf("starting %s: %w", binPath, err)
	}

	stopTail := tailToWriter(opts.SerialLogPath, rep.Writer())
	defer stopTail()

	if opts.Expect == "" {
		err := cmd.Wait()
		if err != nil {
			rep.Fail("deploy", "qemu exited with an error", toolErr.String())
		}
		return err
	}

	waitErr := waitForExpect(opts.SerialLogPath, opts.Expect, opts.Timeout)
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	stopTail()
	if waitErr != nil {
		rep.Fail("deploy", waitErr.Error(), "")
		return waitErr
	}
	rep.Finish("matched "+strconv.Quote(opts.Expect), "on the serial console")
	return nil
}

// tailToWriter streams appended bytes from the file at path to w as they
// are written (best-effort; a missing or not-yet-created file is
// retried), until the returned stop function is called.
func tailToWriter(path string, w io.Writer) (stop func()) {
	done := make(chan struct{})
	go func() {
		var offset int64
		for {
			select {
			case <-done:
				return
			default:
			}
			f, err := os.Open(path)
			if err == nil {
				if _, err := f.Seek(offset, io.SeekStart); err == nil {
					n, _ := io.Copy(w, f)
					offset += n
				}
				f.Close()
			}
			select {
			case <-done:
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}

func waitForExpect(logPath, expect string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		data, _ := os.ReadFile(logPath)
		if bytes.Contains(data, []byte(expect)) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %q on the serial console; log tail:\n%s",
				timeout, expect, tail(data, 4000))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func tail(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[len(data)-n:])
}
