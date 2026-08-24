package migrate

import (
	"embed"
	"io/fs"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration is a single versioned SQL migration.
type Migration struct {
	Version string
	Name    string
	SQL     string
}

// Load reads and orders migration files from the embedded filesystem.
func Load() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	mods := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(strings.TrimSuffix(e.Name(), ".sql"), "_", 2)
		version := parts[0]
		name := e.Name()
		if len(parts) > 1 {
			name = parts[1]
		}
		mods = append(mods, Migration{Version: version, Name: name, SQL: string(raw)})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Version < mods[j].Version })
	return mods, nil
}

// Run applies all pending migrations in order, tracking applied versions in
// the schema_migrations table. Safe to call on every startup.
func Run(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz DEFAULT now()
	)`).Error; err != nil {
		return err
	}

	applied := map[string]bool{}
	rows, err := db.Raw(`SELECT version FROM schema_migrations`).Rows()
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = true
	}
	_ = rows.Close()

	mods, err := Load()
	if err != nil {
		return err
	}

	for _, m := range mods {
		if applied[m.Version] {
			continue
		}
		log.Info().Str("version", m.Version).Str("name", m.Name).Msg("applying migration")
		if err := runTx(db, m.SQL); err != nil {
			return err
		}
		if err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.Version).Error; err != nil {
			return err
		}
	}
	return nil
}

// runTx executes the migration SQL as a single transaction. Statements are
// split on semicolons to allow multiple commands per file.
func runTx(db *gorm.DB, sqlText string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range splitStatements(sqlText) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func splitStatements(s string) []string {
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// AppliedVersions returns the versions already applied (for debugging/health).
func AppliedVersions(db *gorm.DB) ([]string, error) {
	var versions []string
	err := db.Raw(`SELECT version FROM schema_migrations ORDER BY version`).Scan(&versions).Error
	return versions, err
}
