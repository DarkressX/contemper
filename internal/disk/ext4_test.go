package disk_test

import (
	"archive/tar"
	"os/exec"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/contemper-project/contemper/internal/disk"
	"github.com/contemper-project/contemper/internal/hostenv"
	"github.com/contemper-project/contemper/internal/imgtest"
	"github.com/contemper-project/contemper/internal/rootfs"
)

func requireExt4Tools(t *testing.T) (mkfs, debugfs, e2fsck string) {
	t.Helper()
	for _, name := range []string{"mkfs.ext4", "debugfs", "e2fsck"} {
		if hostenv.Find(name) == "" {
			t.Skipf("%s not found; skipping ext4 population test", name)
		}
	}
	return hostenv.Find("mkfs.ext4"), hostenv.Find("debugfs"), hostenv.Find("e2fsck")
}

func debugfsStat(t *testing.T, debugfsPath, img, path string) string {
	t.Helper()
	out, err := exec.Command(debugfsPath, "-R", "stat "+path, img).CombinedOutput()
	if err != nil {
		t.Fatalf("debugfs stat %s: %v\n%s", path, err, out)
	}
	return string(out)
}

func TestPopulateExt4(t *testing.T) {
	_, debugfsPath, e2fsckPath := requireExt4Tools(t)

	files := []imgtest.File{
		{Path: "etc/", Typeflag: tar.TypeDir, Mode: 0o750, Uid: 0, Gid: 0},
		{Path: "etc/hostname", Data: []byte("test-vm\n"), Mode: 0o644, Uid: 0, Gid: 0},
		{Path: "etc/owned", Data: []byte("mine\n"), Mode: 0o600, Uid: 1000, Gid: 1000},
		{Path: "etc/link-to-hostname", Typeflag: tar.TypeSymlink, Linkname: "hostname"},
		{Path: "etc/hardlink-to-hostname", Typeflag: tar.TypeLink, Linkname: "etc/hostname"},
		{Path: "dev/", Typeflag: tar.TypeDir},
		{Path: "dev/null", Typeflag: tar.TypeChar, Devmajor: 1, Devminor: 3, Mode: 0o666},
		{Path: "a file with spaces.txt", Data: []byte("spacey\n")},
	}
	img, err := imgtest.Image(v1.Platform{OS: "linux", Architecture: "amd64"}, nil, files)
	if err != nil {
		t.Fatal(err)
	}
	rfs, err := rootfs.Build(img, nil)
	if err != nil {
		t.Fatalf("rootfs.Build: %v", err)
	}
	defer rfs.Close()

	imgPath := t.TempDir() + "/root.img"
	warnings, err := disk.PopulateExt4(rfs, imgPath, disk.Ext4Options{
		Label:     "contemper-root",
		SizeBytes: 64 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("PopulateExt4: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// e2fsck -fn is the authoritative correctness check.
	out, err := exec.Command(e2fsckPath, "-fn", imgPath).CombinedOutput()
	if err != nil {
		t.Fatalf("e2fsck -fn reported problems: %v\n%s", err, out)
	}

	hostnameStat := debugfsStat(t, debugfsPath, imgPath, "/etc/hostname")
	if !strings.Contains(hostnameStat, "Mode:  0644") {
		t.Errorf("hostname mode: %s", hostnameStat)
	}
	if !strings.Contains(hostnameStat, "Links: 2") {
		t.Errorf("hostname should have 2 links (itself + hardlink): %s", hostnameStat)
	}

	ownedStat := debugfsStat(t, debugfsPath, imgPath, "/etc/owned")
	if !strings.Contains(ownedStat, "User:  1000") && !strings.Contains(ownedStat, "User:1000") && !strings.Contains(ownedStat, "User:     1000") {
		t.Errorf("owned uid not applied: %s", ownedStat)
	}

	linkStat := debugfsStat(t, debugfsPath, imgPath, "/etc/link-to-hostname")
	if !strings.Contains(linkStat, "Type: symlink") || !strings.Contains(linkStat, `Fast link dest: "hostname"`) {
		t.Errorf("symlink not as expected: %s", linkStat)
	}

	devStat := debugfsStat(t, debugfsPath, imgPath, "/dev/null")
	if !strings.Contains(devStat, "Type: character special") || !strings.Contains(devStat, "01:03") {
		t.Errorf("device node not as expected: %s", devStat)
	}

	spaceStat := debugfsStat(t, debugfsPath, imgPath, `"/a file with spaces.txt"`)
	if !strings.Contains(spaceStat, "Type: regular") {
		t.Errorf("file with spaces not as expected: %s", spaceStat)
	}
}
