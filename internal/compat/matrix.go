package compat

import (
	"encoding/json"
	"errors"
	"sort"
)

type State string

const (
	Experimental     State = "experimental"
	Inferred         State = "inferred"
	RuntimeObserved  State = "runtime-observed"
	FixtureConfirmed State = "fixture-confirmed"
	Supported        State = "supported"
	Deprecated       State = "deprecated"
)

type Rule struct {
	ID              string   `json:"id"`
	Profile         string   `json:"profile"`
	State           State    `json:"state"`
	Sources         []string `json:"sources,omitempty"`
	Fixtures        []string `json:"fixtures,omitempty"`
	Runs            []string `json:"oracleRuns,omitempty"`
	Counterexamples []string `json:"counterexamples,omitempty"`
	Owner           string   `json:"owner,omitempty"`
	Version         string   `json:"version"`
}
type Matrix struct {
	Version string `json:"version"`
	Rules   []Rule `json:"rules"`
}

func (m *Matrix) Normalize() {
	sort.Slice(m.Rules, func(i, j int) bool { return m.Rules[i].ID < m.Rules[j].ID })
}
func (m *Matrix) Promote(id string) error {
	for i := range m.Rules {
		if m.Rules[i].ID != id {
			continue
		}
		r := &m.Rules[i]
		if len(r.Counterexamples) > 0 {
			return errors.New("promotion blocked by counterexample")
		}
		if len(r.Fixtures) == 0 || len(r.Runs) == 0 {
			return errors.New("promotion requires fixture and oracle evidence")
		}
		if r.State != FixtureConfirmed && r.State != RuntimeObserved && r.State != Supported {
			return errors.New("promotion requires fixture-confirmed or runtime-observed state")
		}
		r.State = Supported
		return nil
	}
	return errors.New("unknown rule")
}
func (m Matrix) Allowed(id string, strict bool) bool {
	for _, r := range m.Rules {
		if r.ID == id {
			if strict {
				return r.State == Supported
			}
			return r.State != Experimental && r.State != Deprecated
		}
	}
	return false
}
func (m Matrix) JSON() ([]byte, error) { m.Normalize(); return json.Marshal(m) }
