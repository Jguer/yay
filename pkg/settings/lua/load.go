package lua

import (
	"fmt"

	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/text"
)

// LoadInto applies the yay.opt values from path onto cfg.
func LoadInto(_ *text.Logger, path string, cfg any) error {
	engine := New()
	defer engine.Close()

	if err := engine.L.DoFile(path); err != nil {
		return err
	}

	unknown, errs := engine.Apply(cfg)

	if len(unknown) == 0 && len(errs) == 0 {
		return nil
	}

	merr := &multierror.MultiError{}

	for _, key := range unknown {
		merr.Add(fmt.Errorf("init.lua: unknown yay.opt key: %s", key))
	}

	for _, err := range errs {
		merr.Add(fmt.Errorf("init.lua: %w", err))
	}

	return merr.Return()
}
