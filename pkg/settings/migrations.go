package settings

import (
	"fmt"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
)

type configMigration interface {
	// Description of what the migration does
	fmt.Stringer
	// return true if migration was done
	Do(config *Configuration) bool
	// Target version of the migration (e.g. "11.2.1")
	// Should match the version of yay releasing this migration
	TargetVersion() string
}

type configProviderMigration struct{}

func (migration *configProviderMigration) String() string {
	return gotext.Get("Disable 'provides' setting by default")
}

func (migration *configProviderMigration) Do(config *Configuration) bool {
	if config.Provides {
		config.Provides = false

		return true
	}

	return false
}

func (migration *configProviderMigration) TargetVersion() string {
	return "11.2.1"
}

type configSortByMigration struct{}

func (migration *configSortByMigration) String() string {
	return gotext.Get("Reset 'sortby' setting to default")
}

func (migration *configSortByMigration) Do(config *Configuration) bool {
	if config.SortBy != "" {
		config.SortBy = ""

		return true
	}

	return false
}

func (migration *configSortByMigration) TargetVersion() string {
	return "13.0.0"
}

func DefaultMigrations() []configMigration {
	return []configMigration{
		&configProviderMigration{},
		&configSortByMigration{},
	}
}

func (c *Configuration) RunMigrations(logger *text.Logger, migrations []configMigration,
	configPath, newVersion string,
) error {
	saveConfig := false

	for _, migration := range migrations {
		if db.VerCmp(migration.TargetVersion(), c.Version) > 0 {
			if migration.Do(c) {
				logger.Infoln("Config migration executed (",
					migration.TargetVersion(), "):", migration)

				saveConfig = true
			}
		}
	}

	if saveConfig {
		return c.Save(configPath, newVersion)
	}

	return nil
}
