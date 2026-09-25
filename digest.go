package bcg

import (
	"crypto/sha256"
	"encoding/hex"

	formatbcg "github.com/shmatovv/bcg-engine/v2/internal/format/bcg"
)

// Digest is a SHA-256 semantic identity value.
type Digest [sha256.Size]byte

func (d Digest) String() string { return hex.EncodeToString(d[:]) }
func (d Digest) Bytes() []byte  { return append([]byte(nil), d[:]...) }

// SemanticDigest hashes canonical model semantics, excluding version lineage
// and diagnostic metadata.
func (m Model) SemanticDigest() Digest {
	if !m.IsValid() {
		return Digest{}
	}
	b, err := formatbcg.SemanticProjection(m.version)
	if err != nil {
		return Digest{}
	}
	return sha256.Sum256(append([]byte("bcg-engine/v2/semantic-digest/1\x00"), b...))
}
