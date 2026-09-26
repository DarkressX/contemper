package qemu

import (
	"os"
	"strings"
	"testing"
)

func TestBuildArgsWithFirmware(t *testing.T) {
	pflash := writeFakeFile(t, "edk2-fake.fd")
	info := archInfo{
		machine:          "virt",
		pflashCandidates: []string{pflash},
		biosCandidates:   nil,
	}
	args, err := buildArgs(info, Options{
		DiskPath:      "/tmp/bundle/disk.qcow2",
		DiskFormat:    "qcow2",
		SerialLogPath: "/tmp/serial.log",
	})
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-M virt",
		"if=pflash,format=raw,readonly=on,file=" + pflash,
		"file=/tmp/bundle/disk.qcow2,if=virtio,format=qcow2,snapshot=on",
		"-nographic",
		"-serial file:/tmp/serial.log",
		"-m 1G",
		"-smp 2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
}

func TestBuildArgsFallsBackToBIOS(t *testing.T) {
	bios := writeFakeFile(t, "OVMF_CODE-fake.fd")
	info := archInfo{
		machine:          "q35",
		pflashCandidates: []string{"/nonexistent/edk2.fd"},
		biosCandidates:   []string{bios},
	}
	args, err := buildArgs(info, Options{DiskPath: "/tmp/d.qcow2", SerialLogPath: "/tmp/s.log"})
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-bios "+bios) {
		t.Errorf("expected -bios fallback, got %q", joined)
	}
}

func TestBuildArgsNoFirmwareFound(t *testing.T) {
	info := archInfo{machine: "virt"}
	if _, err := buildArgs(info, Options{}); err == nil {
		t.Fatalf("expected an error when no firmware candidate exists")
	}
}

func writeFakeFile(t *testing.T, name string) string {
	t.Helper()
	p := t.TempDir() + "/" + name
	if err := os.WriteFile(p, []byte("fake firmware"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
