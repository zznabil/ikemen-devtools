package doctor

import "encoding/json"

type Service struct{}

func (Service) Status(path string) Report { return Inspect(path) }
func (Service) StatusJSON(path string) (string, error) {
	b, err := json.Marshal(Inspect(path))
	return string(b), err
}
func (Service) Rebuild(path string) error { return Rebuild(path) }
func (Service) Cleanup(path string) error { return Cleanup(path) }
