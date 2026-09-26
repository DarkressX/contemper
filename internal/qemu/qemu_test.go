package qemu_test

import (
	"testing"

	"github.com/contemper-project/contemper/internal/qemu"
)

func TestBuildArgsUnknownArch(t *testing.T) {
	if _, err := qemu.BuildArgs(qemu.Options{Arch: "riscv64"}); err == nil {
		t.Fatalf("expected an error for an unknown arch")
	}
}
