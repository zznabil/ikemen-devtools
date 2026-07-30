package graph

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Kind string

const (
	NodeConfig    Kind = "config"
	NodeMotif     Kind = "motif"
	NodeSelect    Kind = "select"
	NodeCharacter Kind = "character"
	NodeStage     Kind = "stage"
	NodeAsset     Kind = "asset"
	NodeOption    Kind = "option"
)

type Status string

const (
	StatusResolved    Status = "resolved"
	StatusMissing     Status = "missing"
	StatusAmbiguous   Status = "ambiguous"
	StatusConditional Status = "conditional"
)

type Span struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}
type Node struct {
	ID       string `json:"id"`
	Kind     Kind   `json:"kind"`
	Authored string `json:"authored,omitempty"`
	Path     string `json:"path,omitempty"`
	Status   Status `json:"status,omitempty"`
	Span     Span   `json:"span"`
}
type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Syntax     string `json:"syntax"`
	Rule       string `json:"rule"`
	Confidence string `json:"confidence"`
	Status     Status `json:"status,omitempty"`
	Authored   string `json:"authored,omitempty"`
	Span       Span   `json:"span"`
}
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Span    Span   `json:"span"`
}
type Graph struct {
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Build follows select.def and, when present, save/config.json and character [Files].
// Authored paths are retained while Path is normalized for stable identity.
func Build(root, selectPath string) (Graph, error) {
	var g Graph
	root, err := filepath.Abs(root)
	if err != nil {
		return g, err
	}
	sp := selectPath
	if !filepath.IsAbs(sp) {
		sp = filepath.Join(root, sp)
	}
	sp = filepath.Clean(sp)
	data, err := os.ReadFile(sp)
	if err != nil {
		return g, err
	}
	addNode := func(kind Kind, authored, path string, status Status, span Span) Node {
		n := Node{ID: id(kind, filepath.ToSlash(path)), Kind: kind, Authored: authored, Path: path, Status: status, Span: span}
		g.Nodes = append(g.Nodes, n)
		return n
	}
	selectNode := addNode(NodeSelect, selectPath, sp, StatusResolved, Span{sp, 1})
	// Configuration is optional: detect the conventional save/config.json without making it a prerequisite.
	cfgPath := filepath.Join(root, "save", "config.json")
	if cfg, e := os.ReadFile(cfgPath); e == nil {
		cfgNode := addNode(NodeConfig, "save/config.json", cfgPath, StatusResolved, Span{cfgPath, 1})
		g.Edges = append(g.Edges, Edge{cfgNode.ID, selectNode.ID, "config.select", "config-to-select", "exact", StatusResolved, "save/config.json", Span{cfgPath, 1}})
		var obj map[string]any
		if json.Unmarshal(cfg, &obj) == nil {
			for _, key := range []string{"motif", "system", "select"} {
				if raw, ok := obj[key].(string); ok && raw != "" {
					p := raw
					if !filepath.IsAbs(p) {
						p = filepath.Join(filepath.Dir(cfgPath), p)
					}
					k := NodeMotif
					if key == "select" {
						k = NodeSelect
					}
					nstatus := StatusMissing
					if _, e := os.Stat(p); e == nil {
						nstatus = StatusResolved
					}
					n := addNode(k, raw, filepath.Clean(p), nstatus, Span{cfgPath, 1})
					g.Edges = append(g.Edges, Edge{cfgNode.ID, n.ID, "config." + key, "config-to-target", "exact", nstatus, raw, Span{cfgPath, 1}})
				}
			}
		}
	}
	section := ""
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for scan.Scan() {
		line++
		raw := scan.Text()
		v := strings.TrimSpace(raw)
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			section = strings.ToLower(strings.TrimSpace(v[1 : len(v)-1]))
			continue
		}
		if section != "characters" && section != "stages" && section != "extrastages" {
			continue
		}
		if v == "" || strings.HasPrefix(v, ";") || strings.HasPrefix(v, "#") {
			continue
		}
		parts := strings.Split(v, ",")
		authored := strings.TrimSpace(parts[0])
		if authored == "" {
			continue
		}
		kind := NodeCharacter
		if section != "characters" {
			kind = NodeStage
		}
		target := authored
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(sp), target)
		}
		target = filepath.Clean(target)
		status := StatusResolved
		if strings.EqualFold(authored, "randomselect") || strings.EqualFold(authored, "random") {
			status = StatusConditional
		}
		if status == StatusResolved {
			if _, e := os.Stat(target); e != nil {
				status = StatusMissing
			}
		}
		n := addNode(kind, authored, target, status, Span{sp, line})
		conf := "exact"
		if status != StatusResolved {
			conf = "inferred"
			code := "missing-target"
			if status == StatusConditional {
				code = "conditional-target"
			}
			g.Diagnostics = append(g.Diagnostics, Diagnostic{Code: code, Message: "roster target " + authored + " is " + string(status), Span: Span{sp, line}})
		}
		g.Edges = append(g.Edges, Edge{selectNode.ID, n.ID, "select." + section, "roster-entry", conf, status, authored, Span{sp, line}})
		for _, opt := range parts[1:] {
			o := strings.TrimSpace(opt)
			if o == "" {
				continue
			}
			on := addNode(NodeOption, o, o, StatusResolved, Span{sp, line})
			g.Edges = append(g.Edges, Edge{n.ID, on.ID, "select-option", "row-option", "exact", StatusResolved, o, Span{sp, line}})
		}
		if kind == NodeCharacter && status == StatusResolved {
			g.addManifestEdges(n, target)
		}
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})
	return g, nil
}
func (g *Graph) addManifestEdges(character Node, def string) {
	b, e := os.ReadFile(def)
	if e != nil {
		return
	}
	sec := ""
	line := 0
	s := bufio.NewScanner(strings.NewReader(string(b)))
	for s.Scan() {
		line++
		v := strings.TrimSpace(s.Text())
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			sec = strings.ToLower(strings.Trim(v, "[]"))
			continue
		}
		if sec != "files" || v == "" || strings.HasPrefix(v, ";") {
			continue
		}
		p := strings.SplitN(v, "=", 2)
		if len(p) != 2 {
			continue
		}
		authored := strings.TrimSpace(p[1])
		if authored == "" {
			continue
		}
		target := authored
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(def), target)
		}
		target = filepath.Clean(target)
		status := StatusResolved
		if _, e := os.Stat(target); e != nil {
			status = StatusMissing
		}
		n := Node{ID: id(NodeAsset, target), Kind: NodeAsset, Authored: authored, Path: target, Status: status, Span: Span{def, line}}
		g.Nodes = append(g.Nodes, n)
		g.Edges = append(g.Edges, Edge{character.ID, n.ID, "def.files", "character-asset", map[bool]string{true: "exact", false: "inferred"}[status == StatusResolved], status, authored, Span{def, line}})
	}
}
func id(k Kind, p string) string {
	h := sha256.Sum256([]byte(string(k) + ":" + filepath.ToSlash(filepath.Clean(p))))
	return fmt.Sprintf("%s:%s", k, hex.EncodeToString(h[:8]))
}
