package bcg

import (
	"crypto/sha256"

	formatbcg "github.com/shmatovv/bcg-engine/v2/internal/format/bcg"
)

// ContentDigest is the SHA-256 semantic identity of Dataset runtime content.
// It excludes DatasetID and model schema/version lineage. The canonical
// projection includes entities, relations (including applicability), and all
// object bindings with their typed values, units, scopes, periods and source
// series data.
func (d Dataset) ContentDigest() Digest {
	if !d.IsValid() {
		return Digest{}
	}
	b, err := formatbcg.MarshalDatasetContent(d.value)
	if err != nil {
		return Digest{}
	}
	return sha256.Sum256(append([]byte("bcg-engine/v2/dataset-content-digest/1\x00"), b...))
}
