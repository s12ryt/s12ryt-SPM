package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Runtime struct {
	Active      string `json:"active"`
	Pending     string `json:"pending,omitempty"`
	MigrationID string `json:"migrationId,omitempty"`
	Path        string `json:"-"`
	Environment bool   `json:"-"`
	LastError   string `json:"-"`
}

func Load(ctx context.Context, dir, environment string) (*Store, *Runtime, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, nil, err
	}
	r := &Runtime{Active: "sqlite:///" + filepath.ToSlash(filepath.Join(abs, "spm.db")), Path: filepath.Join(abs, "database.json")}
	defaults := *r
	b, err := os.ReadFile(r.Path)
	if err == nil {
		if err = json.Unmarshal(b, r); err != nil {
			if environment == "" {
				return nil, nil, errors.New("invalid database.json")
			}
			*r = defaults
			r.LastError = "Web 資料庫設定無法解析，目前依 DATABASE_URL 啟動。"
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		if environment == "" {
			return nil, nil, err
		}
		r.LastError = "Web 資料庫設定無法讀取，目前依 DATABASE_URL 啟動。"
	}
	if environment != "" {
		r.Environment = true
		r.Active = environment
		s, err := Open(ctx, environment)
		return s, r, err
	}
	s, err := Open(ctx, r.Active)
	if err != nil {
		return nil, r, err
	}
	if r.Pending == "" {
		return s, r, nil
	}
	migrateCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	target, err := Open(migrateCtx, r.Pending)
	if err != nil {
		r.LastError = "無法連線至目標資料庫，繼續使用原資料庫。"
		return s, r, nil
	}
	if err = Copy(migrateCtx, s, target, r.MigrationID); err != nil {
		target.Close()
		r.LastError = "搬移失敗：目標須為空資料庫，且連線與磁碟空間須可用。原資料已保留。"
		return s, r, nil
	}
	next := *r
	next.Active = r.Pending
	next.Pending = ""
	next.MigrationID = ""
	if err = next.persist(); err != nil {
		target.Close()
		r.LastError = "搬移完成但無法儲存設定，繼續使用原資料庫；下次重啟將重新同步。"
		return s, r, nil
	}
	s.Close()
	return target, &next, nil
}
func (r *Runtime) persist() error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(r.Path), ".database-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, r.Path)
}
func (r *Runtime) Schedule(raw string) error {
	if r.Environment {
		return errors.New("DATABASE_URL is set; remove it before scheduling a database migration")
	}
	next := *r
	if raw == "" {
		next.Pending = ""
		next.MigrationID = ""
	} else {
		if err := ValidateURL(raw); err != nil {
			return err
		}
		if raw == r.Active {
			return errors.New("destination is already active")
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return err
		}
		next.Pending = raw
		next.MigrationID = hex.EncodeToString(id[:])
	}
	if err := next.persist(); err != nil {
		return err
	}
	*r = next
	return nil
}
