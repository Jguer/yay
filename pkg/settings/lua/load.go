package lua

import (
	"fmt"

	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/text"
)

// Load loads path, applies its yay.opt values onto cfg, and returns the live engine.
func Load(_ *text.Logger, path string, cfg any) (*Engine, error) {
	engine := New()

	if err := engine.L.DoFile(path); err != nil {
		engine.Close()
		return nil, err
	}

	unknown, errs := engine.Apply(cfg)

	if len(unknown) == 0 && len(errs) == 0 {
		return engine, nil
	}

	merr := &multierror.MultiError{}

	for _, key := range unknown {
		merr.Add(fmt.Errorf("init.lua: unknown yay.opt key: %s", key))
	}

	for _, err := range errs {
		merr.Add(fmt.Errorf("init.lua: %w", err))
	}

	if err := merr.Return(); err != nil {
		engine.Close()
		return nil, err
	}

	return engine, nil
}

// LoadInto applies the yay.opt values from path onto cfg.
func LoadInto(logger *text.Logger, path string, cfg any) error {
	engine, err := Load(logger, path, cfg)
	if engine != nil {
		defer engine.Close()
	}

	return err
}
