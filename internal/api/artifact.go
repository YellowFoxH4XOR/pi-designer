package api

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/akki/pidesigner/internal/project"
)

// ArtifactFile transports file bytes without lossy binary-to-string conversion.
type ArtifactFile struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	MIMEType string `json:"mimeType"`
}

// ArtifactBundle is the full set of files an artifact needs.
type ArtifactBundle struct {
	EntryFile string                  `json:"entryFile"`
	Files     map[string]ArtifactFile `json:"files"`
}

// BuildArtifact collects the entry file plus every local asset it references.
func BuildArtifact(pm *project.Manager, entry string) (*ArtifactBundle, error) {
	if entry == "" {
		entry = pm.FindEntryFile()
	}
	if entry == "" {
		return nil, fmt.Errorf("no artifact entry file found")
	}
	entryPath, local, err := canonicalArtifactPath(entry, "")
	if err != nil {
		return nil, fmt.Errorf("invalid artifact entry %q: %w", entry, err)
	}
	if !local {
		return nil, fmt.Errorf("artifact entry %q is not a local project file", entry)
	}
	entryBytes, err := pm.ReadBytes(entryPath)
	if err != nil {
		return nil, fmt.Errorf("artifact entry %q: %w", entryPath, err)
	}
	files := map[string]ArtifactFile{entryPath: artifactFile(entryPath, entryBytes)}

	refs, err := referencedAssets(string(entryBytes), entryPath)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if _, ok := files[ref]; ok {
			continue
		}
		data, err := pm.ReadBytes(ref)
		if err != nil {
			return nil, fmt.Errorf("artifact dependency %q referenced by %q: %w", ref, entryPath, err)
		}
		files[ref] = artifactFile(ref, data)
	}
	return &ArtifactBundle{EntryFile: entryPath, Files: files}, nil
}

func artifactFile(filePath string, data []byte) ArtifactFile {
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	if parsed, _, err := mime.ParseMediaType(mediaType); err == nil {
		mediaType = parsed
	}

	if utf8.Valid(data) && textualArtifactMIME(mediaType) {
		return ArtifactFile{Encoding: "utf-8", Content: string(data), MIMEType: mediaType}
	}
	return ArtifactFile{
		Encoding: "base64",
		Content:  base64.StdEncoding.EncodeToString(data),
		MIMEType: mediaType,
	}
}

func textualArtifactMIME(mediaType string) bool {
	switch {
	case strings.HasPrefix(mediaType, "text/"):
		return true
	case mediaType == "application/json",
		mediaType == "application/javascript",
		mediaType == "application/x-javascript",
		mediaType == "application/xml",
		mediaType == "application/xhtml+xml",
		mediaType == "image/svg+xml":
		return true
	default:
		return false
	}
}

// canonicalArtifactPath resolves a URL-like project reference. Query strings
// and fragments do not identify distinct files in the project tree.
func canonicalArtifactPath(raw, entryDir string) (string, bool, error) {
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if ref.Scheme != "" || ref.Host != "" {
		return "", false, nil
	}
	target := ref.Path
	if target == "" {
		return "", false, nil
	}
	if strings.HasPrefix(target, "/") {
		target = strings.TrimLeft(target, "/")
	} else if entryDir != "" {
		target = path.Join(entryDir, target)
	}
	target = path.Clean(target)
	if target == "." || target == ".." || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
		return "", true, fmt.Errorf("path escapes the project directory")
	}
	return target, true, nil
}

// Attributes are whitespace-separated, so requiring leading whitespace skips
// data-src and script property writes such as img.src = "...".
var artifactReferencePattern = regexp.MustCompile(`(?i)(?:^|\s)(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"'` + "`" + `]+))`)

// artifactNonMarkup matches comments and inline script bodies; neither holds
// attributes, though their text can look like them. A script's own src stays
// in the opening tag, which is kept.
var artifactNonMarkup = regexp.MustCompile(`(?is)<!--.*?-->|(<script\b[^>]*>).*?(</script\s*>)`)

// referencedAssets extracts canonical local href/src targets from an HTML doc.
func referencedAssets(html, entryPath string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	entryDir := path.Dir(entryPath)
	html = artifactNonMarkup.ReplaceAllString(html, "$1$2")
	for _, match := range artifactReferencePattern.FindAllStringSubmatch(html, -1) {
		raw := match[1]
		if raw == "" {
			raw = match[2]
		}
		if raw == "" {
			raw = match[3]
		}
		if strings.TrimSpace(raw) == "" {
			continue
		}
		target, local, err := canonicalArtifactPath(raw, entryDir)
		if err != nil {
			return nil, fmt.Errorf("artifact reference %q: %w", raw, err)
		}
		if !local || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out, nil
}
