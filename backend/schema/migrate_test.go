package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationPrefixUnique(t *testing.T) {
	files, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("Failed to read migrations directory: %v", err)
	}

	prefixes := make(map[string]string)

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		name := file.Name()
		if len(name) < 4 {
			t.Errorf("Migration file name too short: %s", name)
			continue
		}

		prefix := name[:4]
		if existingFile, ok := prefixes[prefix]; ok {
			t.Errorf("Duplicate migration prefix %s found in %s and %s", prefix, existingFile, name)
		} else {
			prefixes[prefix] = name
		}
	}
}
