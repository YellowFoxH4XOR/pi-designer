package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesignSystemContextResolvesOnlyCatalogEntries(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "content")
	for dir, body := range map[string]string{
		filepath.Join(root, "design-systems", "calm"): "# Calm\n",
		filepath.Join(base, "outside"):                "# Outside\n",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := cat.DesignSystemContext("calm")
	if err != nil || ctx.DesignMD != "# Calm\n" || ctx.Dir != filepath.Join(root, "design-systems", "calm") {
		t.Fatalf("catalog entry: %+v, %v", ctx, err)
	}
	for _, id := range []string{"../../outside", "../calm", "/etc"} {
		if ctx, err := cat.DesignSystemContext(id); err == nil {
			t.Fatalf("DesignSystemContext(%q) escaped the catalog: %+v", id, ctx)
		}
	}
}
