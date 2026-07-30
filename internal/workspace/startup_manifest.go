package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolveStartupManifest follows the files the game loads before gameplay. It is
// deliberately bounded by seen paths and MaxDepth-like recursion (64).
func resolveStartupManifest(root string) ([]string, []string) {
	root, _ = filepath.Abs(root)
	seen := map[string]bool{}
	var out []string
	var diags []string
	add := func(path, from string) string {
		p, amb := resolveManifestPath(root, path, from)
		if amb {
			diags = append(diags, fmt.Sprintf("ambiguous-manifest-path:%s", path))
		}
		if p == "" {
			diags = append(diags, fmt.Sprintf("missing-manifest-file:%s", path))
			return ""
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
		return p
	}
	addCharacter := func(path string) string {
		path = strings.Trim(strings.TrimSpace(path), "\"'")
		path = strings.TrimPrefix(path, "=")
		switch strings.ToLower(path) {
		case "", "empty", "randomselect":
			return ""
		}
		try := []string{path}
		if !strings.EqualFold(filepath.Ext(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/"))), ".def") {
			clean := strings.TrimSuffix(strings.ReplaceAll(path, "\\", "/"), "/")
			name := filepath.Base(filepath.FromSlash(clean))
			try = append([]string{clean + "/" + name + ".def", clean + ".def"}, try...)
		}
		for _, candidate := range try {
			if p, amb := resolveManifestPath(root, candidate, filepath.Join(root, "chars")); p != "" {
				if amb {
					diags = append(diags, fmt.Sprintf("ambiguous-manifest-path:%s", path))
				}
				rel, _ := filepath.Rel(root, p)
				rel = filepath.ToSlash(rel)
				if !seen[rel] {
					seen[rel] = true
					out = append(out, rel)
				}
				return p
			}
		}
		diags = append(diags, fmt.Sprintf("missing-manifest-file:%s", path))
		return ""
	}

	var walk func(string, int)
	walk = func(path string, depth int) {
		if depth > 64 || path == "" {
			return
		}
		key := strings.ToLower(filepath.Clean(path))
		if seen["@"+key] {
			return
		}
		seen["@"+key] = true
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		kind := strings.ToLower(filepath.Ext(path))
		if kind != ".def" {
			return
		}
		sections := parseManifestSections(string(b))
		base := filepath.Dir(path)
		for sec, lines := range sections {
			low := strings.ToLower(sec)
			if low == "files" {
				for k, v := range lines {
					kl := strings.ToLower(k)
					if kl == "select" || kl == "fight" || kl == "fonts" || kl == "assets" || kl == "cmd" || kl == "command" || kl == "cns" || kl == "st" || kl == "stcommon" || kl == "air" || kl == "sff" || kl == "snd" || strings.Contains(kl, "asset") {
						for _, ref := range splitRefs(v) {
							if kl == "stcommon" && strings.EqualFold(filepath.Base(filepath.FromSlash(strings.ReplaceAll(ref, "\\", "/"))), "common1.cns") {
								if p, _ := resolveManifestPath(root, ref, base); p == "" {
									continue
								}
							}
							if p := add(ref, base); p != "" {
								walk(p, depth+1)
							}
						}
					}
				}
			}
		}
	}
	// save/config.json is always the authoritative bootstrap source.
	cfgPath := filepath.Join(root, "save", "config.json")
	if b, err := os.ReadFile(cfgPath); err == nil {
		var raw map[string]any
		if json.Unmarshal(b, &raw) == nil {
			for key, value := range raw {
				switch strings.ToLower(key) {
				case "system", "motif", "startstage", "trainingchar", "commonstates":
					for _, s := range jsonStrings(value) {
						if p := add(s, root); p != "" {
							walk(p, 0)
						} else if strings.EqualFold(key, "trainingchar") {
							if p := add(s+".def", root); p != "" {
								walk(p, 0)
							}
						}
					}
				}
			}
		}
	}
	selectPath := add("data/select.def", root)
	if selectPath == "" {
		selectPath = add("select.def", root)
	}
	if selectPath != "" {
		sections := parseManifestSectionLines(readFile(selectPath))
		base := filepath.Dir(selectPath)
		for sec, lines := range sections {
			low := strings.ToLower(sec)
			if low != "characters" && low != "stages" && low != "extrastages" {
				continue
			}
			for _, line := range lines {
				refs := splitRefs(line)
				if len(refs) == 0 {
					continue
				}
				if low == "characters" {
					if p := addCharacter(refs[0]); p != "" {
						walk(p, 0)
					}
					if len(refs) > 1 && (strings.Contains(strings.ToLower(refs[1]), ".def") || strings.Contains(refs[1], "/") || strings.Contains(refs[1], "\\")) {
						if p := add(refs[1], base); p != "" {
							walk(p, 0)
						}
					}
					continue
				}
				if p := add(refs[0], base); p != "" {
					walk(p, 0)
				}
			}
		}
	}
	sort.Strings(out)
	sort.Strings(diags)
	return out, uniqueStrings(diags)
}
func jsonStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		out := []string{}
		for _, item := range x {
			out = append(out, jsonStrings(item)...)
		}
		return out
	default:
		return nil
	}
}

func readFile(path string) string { b, _ := os.ReadFile(path); return string(b) }
func uniqueStrings(in []string) []string {
	out := []string{}
	for _, v := range in {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
func splitRefs(v string) []string {
	v = strings.TrimSpace(strings.SplitN(v, ";", 2)[0])
	v = strings.Trim(v, "\"'")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"'")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func parseManifestSections(text string) map[string]map[string]string {
	out := map[string]map[string]string{}
	sec := ""
	out[sec] = map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			sec = strings.TrimSpace(line[1:strings.Index(line, "]")])
			out[sec] = map[string]string{}
			continue
		}
		if i := strings.Index(line, "="); i >= 0 {
			out[sec][strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		} else {
			out[sec][fmt.Sprintf("_%d", len(out[sec]))] = line
		}
	}
	return out
}
func parseManifestSectionLines(text string) map[string][]string {
	out := map[string][]string{}
	sec := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			sec = strings.TrimSpace(line[1:strings.Index(line, "]")])
			continue
		}
		out[sec] = append(out[sec], line)
	}
	return out
}

func resolveManifestPath(root, raw, declaring string) (string, bool) {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'")
	if raw == "" {
		return "", false
	}
	variants := []string{raw}
	if strings.HasPrefix(raw, "=") {
		variants = append(variants, strings.TrimPrefix(raw, "="))
	}
	for _, variant := range variants {
		variant = strings.ReplaceAll(variant, "\\", string(filepath.Separator))
		candidates := []string{}
		if filepath.IsAbs(variant) {
			candidates = []string{variant}
		} else {
			candidates = []string{filepath.Join(declaring, variant), filepath.Join(root, "data", variant), filepath.Join(root, variant)}
		}
		found := []string{}
		for _, c := range candidates {
			if p := caseInsensitiveExisting(c); p != "" {
				if withinPath(root, p) {
					found = append(found, p)
				} else {
					return "", false
				}
			}
		}
		found = uniqueStrings(found)
		if len(found) > 0 {
			return found[0], len(found) > 1
		}
	}
	return "", false
}
func withinPath(root, p string) bool {
	r, _ := filepath.Abs(root)
	q, _ := filepath.Abs(p)
	r = strings.ToLower(filepath.Clean(r))
	q = strings.ToLower(filepath.Clean(q))
	return q == r || strings.HasPrefix(q, strings.TrimRight(r, string(filepath.Separator))+string(filepath.Separator))
}
func caseInsensitiveExisting(path string) string {
	if p, err := filepath.Abs(filepath.Clean(path)); err == nil {
		if _, e := os.Stat(p); e == nil {
			return p
		}
		parts := []string{}
		cur := p
		for {
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			parts = append(parts, filepath.Base(cur))
			cur = parent
			if _, e := os.Stat(cur); e == nil {
				for i := len(parts) - 1; i >= 0; i-- {
					ents, _ := os.ReadDir(cur)
					want := strings.ToLower(parts[i])
					match := ""
					for _, en := range ents {
						if strings.ToLower(en.Name()) == want {
							match = en.Name()
							break
						}
					}
					if match == "" {
						return ""
					}
					cur = filepath.Join(cur, match)
				}
				return cur
			}
		}
	}
	return ""
}
