package cli

import "github.com/enea-scaccabarozzi/ness/internal/config"

// requireFeature loads the project and refuses a command of a feature that is off, naming the
// feature (R-CFG-054). The set is the union of ness.toml and the profiles (R-PROF-015).
func requireFeature(d Deps, name string) (*project, *config.Config, error) {
	p, err := loadProject(d)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, nil, err
	}
	if !cfg.On(name) {
		return nil, nil, refuse("feature %s is off: add %q to features in %s", name, name, config.Path)
	}
	return p, cfg, nil
}
