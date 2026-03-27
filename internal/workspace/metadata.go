package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

const metadataFile = ".rws.toml"

type Metadata struct {
	CreatedAt time.Time `toml:"created_at"`
	UpdatedAt time.Time `toml:"updated_at"`
}

func WriteMetadata(wsDir string, meta Metadata) error {
	data, err := toml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	return os.WriteFile(filepath.Join(wsDir, metadataFile), data, 0o644)
}

func ReadMetadata(wsDir string) (Metadata, error) {
	data, err := os.ReadFile(filepath.Join(wsDir, metadataFile))
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := toml.Unmarshal(data, &meta); err != nil {
		return Metadata{}, fmt.Errorf("parsing metadata: %w", err)
	}
	return meta, nil
}
