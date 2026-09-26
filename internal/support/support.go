// Package support checks a support image's unconditional file
// requirements against a merged rootfs. Support-image variants and
// branches are out of scope for the MVP; a support image is treated as a
// plain rootfs overlay plus this one annotation.
package support

import (
	"fmt"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/contemper-project/contemper/internal/rootfs"
)

// RequiresFilesAnnotation names a comma-separated list of absolute paths
// that must exist in the merged rootfs, set on the support image's own
// manifest.
const RequiresFilesAnnotation = "io.contemper.requires.files"

// CheckRequires reads RequiresFilesAnnotation off manifest (if present)
// and verifies every listed path exists in rfs.
func CheckRequires(manifest *v1.Manifest, rfs *rootfs.Rootfs) error {
	if manifest == nil {
		return nil
	}
	raw, ok := manifest.Annotations[RequiresFilesAnnotation]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	var missing []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := rfs.Resolve(p); err != nil {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("support image requires missing paths: %s", strings.Join(missing, ", "))
	}
	return nil
}
