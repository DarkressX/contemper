package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"

	"github.com/contemper-project/contemper/internal/bundle"
	"github.com/contemper-project/contemper/internal/progress"
	"github.com/contemper-project/contemper/internal/qemu"
	"github.com/contemper-project/contemper/internal/rootfs"
	"github.com/contemper-project/contemper/internal/source"
	"github.com/contemper-project/contemper/internal/support"
	"github.com/contemper-project/contemper/internal/target"
	"github.com/contemper-project/contemper/internal/validate"
)

type convertOptions struct {
	sourceRef    string
	target       string
	supportRef   string
	outDir       string
	arch         string
	rootSize     string
	keepRaw      bool
	quiet        bool
	verbose      bool
	progressMode string
}

type deployOptions struct {
	bundleDir    string
	to           string
	serialLog    string
	expect       string
	timeout      string
	quiet        bool
	verbose      bool
	progressMode string
}

func newReporter(w *os.File, modeStr string, verbose, quiet bool) (*progress.Reporter, error) {
	mode, err := progress.ParseMode(modeStr)
	if err != nil {
		return nil, err
	}
	return progress.New(w, mode, verbose, quiet), nil
}

func runConvert(cmd *cobra.Command, opts convertOptions) error {
	rep, err := newReporter(os.Stderr, opts.progressMode, opts.verbose, opts.quiet)
	if err != nil {
		return err
	}

	canonicalTarget, asm, err := target.Resolve(opts.target)
	if err != nil {
		return err
	}

	platform, err := source.HostPlatform(opts.arch)
	if err != nil {
		return err
	}

	var rootSizeBytes int64
	if opts.rootSize != "" {
		rootSizeBytes, err = parseSize(opts.rootSize)
		if err != nil {
			return fmt.Errorf("--root-size: %w", err)
		}
	}

	ref, err := source.ParseRef(opts.sourceRef)
	if err != nil {
		return err
	}
	img, err := source.Load(ref, platform)
	if err != nil {
		rep.Fail("resolve source", err.Error(), "")
		return err
	}
	defer img.Close()

	rep.Line("📦", opts.sourceRef, platform.String())

	cfg, err := img.Image.ConfigFile()
	if err != nil {
		rep.Fail("readiness check", err.Error(), "")
		return fmt.Errorf("reading image config: %w", err)
	}
	if err := source.CheckReady(cfg); err != nil {
		rep.Fail("readiness check", err.Error(), "")
		return err
	}
	rep.Line("✅", "contemper-ready", "")

	if opts.target == canonicalTarget {
		rep.Line("🎯", "target "+canonicalTarget, "(explicit, no alias)")
	} else {
		rep.Line("🎯", fmt.Sprintf("target %s → %s", opts.target, canonicalTarget), "")
	}
	if !img.Reproducible {
		rep.Warn("local source", "bundle is not reproducible")
	}
	rep.Blank()

	var supportImg *source.Image
	var supportRefStr string
	if opts.supportRef != "" {
		supportRef, err := source.ParseRef(opts.supportRef)
		if err != nil {
			return err
		}
		supportRefStr = supportRef.String()
		supportImg, err = source.Load(supportRef, platform)
		if err != nil {
			rep.Fail("support image", err.Error(), "")
			return fmt.Errorf("loading support image: %w", err)
		}
		defer supportImg.Close()

		supportLayers, err := supportImg.Image.Layers()
		if err != nil {
			return fmt.Errorf("reading support image layers: %w", err)
		}
		rep.Line("🧩", "support image · "+opts.supportRef, "")
		rep.Sub("✔", "rootfs", fmt.Sprintf("%d layers", len(supportLayers)))
		rep.Blank()
	}

	sourceLayers, err := img.Image.Layers()
	if err != nil {
		return fmt.Errorf("reading source image layers: %w", err)
	}
	mergeLabel := fmt.Sprintf("merging %d %s", len(sourceLayers), pluralize(len(sourceLayers), "layer"))
	if supportImg != nil {
		supportLayers, _ := supportImg.Image.Layers()
		mergeLabel = fmt.Sprintf("merging %d + %d layers", len(sourceLayers), len(supportLayers))
	}
	mergeStage := rep.BeginStage("🧬", mergeLabel)

	rfs, err := buildRootfs(img, supportImg)
	if err != nil {
		mergeStage.Fail("merge", err.Error(), "")
		return err
	}
	defer rfs.Close()
	mergeStage.Done("🧬", mergeLabel, "")

	if supportImg != nil {
		manifest, err := supportImg.Image.Manifest()
		if err != nil {
			return fmt.Errorf("reading support image manifest: %w", err)
		}
		if err := support.CheckRequires(manifest, rfs); err != nil {
			rep.Fail("support image", err.Error(), "")
			return err
		}
	}

	val, err := validate.Validate(rfs)
	if err != nil {
		rep.Fail("validate", err.Error(), "a contemper-ready image must provide a kernel, initrd, cmdline and init at the fixed paths")
		return err
	}
	reportFixedPaths(rep, rfs, val)
	rep.Line("✅", "requirements satisfied", "kernel · initrd · init")
	rep.Blank()

	bundleName := fmt.Sprintf("%s-%s.%s", img.RepoBase, img.Tag, machineArch(platform.Architecture))
	outBundleDir := filepath.Join(opts.outDir, bundleName)
	if err := os.MkdirAll(outBundleDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", outBundleDir, err)
	}

	diskInfo, warnings, err := asm.Assemble(rfs, val, platform.Architecture, outBundleDir, target.Options{
		RootSizeBytes: rootSizeBytes,
		KeepRaw:       opts.keepRaw,
		Progress:      rep,
	})
	if err != nil {
		return err
	}
	for _, w := range warnings {
		rep.Warn("warning", w)
	}

	manifest := &bundle.Manifest{
		FormatVersion:    bundle.FormatVersion,
		ContemperVersion: version,
		CreatedAt:        time.Now().UTC(),
		Source: bundle.ImageRef{
			Ref:    ref.String(),
			Digest: img.Digest.String(),
		},
		Target: canonicalTarget,
		Arch:   platform.Architecture,
		Disk: bundle.DiskInfo{
			File:      diskInfo.Filename,
			Format:    diskInfo.Format,
			SizeBytes: diskInfo.SizeBytes,
			SHA256:    diskInfo.SHA256,
		},
		Volumes:      bundle.SortedKeys(cfg.Config.Volumes),
		Hints:        hintsFrom(cfg),
		Reproducible: img.Reproducible,
	}
	if supportImg != nil {
		manifest.Support = &bundle.ImageRef{Ref: supportRefStr, Digest: supportImg.Digest.String()}
	}

	if err := bundle.Write(outBundleDir, manifest); err != nil {
		return err
	}

	rep.Blank()
	rep.Line("📦", bundleName+"/", "disk + manifest")
	elapsed := rep.Elapsed().Round(100 * time.Millisecond)
	rep.Finish("bundle ready", fmt.Sprintf("%-16s%s", progress.HumanBytes(diskInfo.SizeBytes), elapsed))

	fmt.Fprintln(cmd.OutOrStdout(), outBundleDir)
	return nil
}

// reportFixedPaths prints one ✔ sub-line per fixed path, showing what a
// symlink resolved to and the file's size, mirroring the doc's
// "✔ /boot/contemper/vmlinuz → /boot/vmlinuz-virt  9.8 MiB" example.
func reportFixedPaths(rep *progress.Reporter, rfs *rootfs.Rootfs, val *validate.Result) {
	reportOne := func(fixedPath string, size int64) {
		e, err := rfs.Resolve(fixedPath)
		if err != nil {
			return
		}
		label := fixedPath
		if e.Path != fixedPath {
			label = fixedPath + " → " + e.Path
		}
		rep.Sub("✔", label, progress.HumanBytes(size))
	}
	reportOne(validate.KernelPath, int64(len(val.Kernel)))
	reportOne(validate.InitrdPath, int64(len(val.Initrd)))
	rep.Sub("✔", validate.CmdlinePath, strconv.Quote(val.Cmdline))
	rep.Sub("✔", validate.InitPath, "")
	if val.OSRelease != nil {
		rep.Sub("✔", validate.OSReleasePath, "")
	}
}

// buildRootfs adapts the two source.Image handles to rootfs.Build's
// v1.Image parameters (a small helper mainly to keep runConvert linear).
func buildRootfs(img, supportImg *source.Image) (*rootfs.Rootfs, error) {
	if supportImg == nil {
		return rootfs.Build(img.Image, nil)
	}
	return rootfs.Build(img.Image, supportImg.Image)
}

func hintsFrom(cfg *v1.ConfigFile) bundle.Hints {
	h := bundle.Hints{ExposedPorts: bundle.SortedKeys(cfg.Config.ExposedPorts)}
	if hc := cfg.Config.Healthcheck; hc != nil {
		h.Healthcheck = &bundle.Healthcheck{
			Test:        hc.Test,
			Interval:    hc.Interval.String(),
			Timeout:     hc.Timeout.String(),
			StartPeriod: hc.StartPeriod.String(),
			Retries:     hc.Retries,
		}
	}
	return h
}

func runDeploy(cmd *cobra.Command, opts deployOptions) error {
	rep, err := newReporter(os.Stderr, opts.progressMode, opts.verbose, opts.quiet)
	if err != nil {
		return err
	}

	if opts.to != "local-qemu" {
		return fmt.Errorf("unknown deploy target %q (only local-qemu is supported)", opts.to)
	}

	manifest, err := bundle.Read(opts.bundleDir)
	if err != nil {
		return err
	}

	var timeout time.Duration
	if opts.timeout != "" {
		timeout, err = time.ParseDuration(opts.timeout)
		if err != nil {
			return fmt.Errorf("--timeout: %w", err)
		}
	}

	return qemu.Deploy(qemu.Options{
		Arch:          manifest.Arch,
		DiskPath:      filepath.Join(opts.bundleDir, manifest.Disk.File),
		DiskFormat:    manifest.Disk.Format,
		SerialLogPath: opts.serialLog,
		Expect:        opts.expect,
		Timeout:       timeout,
		Progress:      rep,
	})
}

// pluralize returns unit or unit+"s" depending on n, for the common case
// of a regular plural.
func pluralize(n int, unit string) string {
	if n == 1 {
		return unit
	}
	return unit + "s"
}

// parseSize parses a human size like "2GiB", "512MiB", "1073741824" into
// bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
		{"GB", 1e9},
		{"MB", 1e6},
		{"KB", 1e3},
		{"B", 1},
	}
	for _, sfx := range suffixes {
		if strings.HasSuffix(strings.ToUpper(s), strings.ToUpper(sfx.suffix)) {
			numStr := s[:len(s)-len(sfx.suffix)]
			n, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size %q", s)
			}
			return int64(n * float64(sfx.mult)), nil
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n, nil
}

// machineArch maps an OCI architecture to the machine name used in bundle
// directory names (as in uname -m and the UEFI/qemu naming).
func machineArch(ociArch string) string {
	switch ociArch {
	case "arm64":
		return "aarch64"
	case "amd64":
		return "x86_64"
	}
	return ociArch
}
