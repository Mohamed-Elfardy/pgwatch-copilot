package profile

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type WorkloadType string

const (
	WorkloadOLTP    WorkloadType = "OLTP"
	WorkloadOLAP    WorkloadType = "OLAP"
	WorkloadMixed   WorkloadType = "Mixed"
	WorkloadUnknown WorkloadType = "Unknown"
)

type ClusterProfile struct {
	SysID                 string
	WorkloadType          WorkloadType
	HistoricalBottlenecks []string
	Notes                 string
	UpdatedAt             time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(connStr string) (*Store, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS copilot_cluster_profiles (
			sys_id                 TEXT PRIMARY KEY,
			workload_type          TEXT NOT NULL DEFAULT 'Unknown',
			historical_bottlenecks TEXT[] NOT NULL DEFAULT '{}',
			notes                  TEXT NOT NULL DEFAULT '',
			updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (s *Store) Get(sysID string) (*ClusterProfile, error) {
	p := &ClusterProfile{SysID: sysID}
	var bottlenecks []string

	err := s.db.QueryRow(`
		SELECT workload_type, historical_bottlenecks, notes, updated_at
		FROM copilot_cluster_profiles
		WHERE sys_id = $1
	`, sysID).Scan(&p.WorkloadType, pq.Array(&bottlenecks), &p.Notes, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		p.WorkloadType = WorkloadUnknown
		p.HistoricalBottlenecks = []string{}
		if err := s.Save(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	p.HistoricalBottlenecks = bottlenecks
	return p, nil
}

func (s *Store) Save(p *ClusterProfile) error {
	_, err := s.db.Exec(`
		INSERT INTO copilot_cluster_profiles (sys_id, workload_type, historical_bottlenecks, notes, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (sys_id) DO UPDATE SET
			workload_type          = EXCLUDED.workload_type,
			historical_bottlenecks = EXCLUDED.historical_bottlenecks,
			notes                  = EXCLUDED.notes,
			updated_at             = NOW()
	`, p.SysID, string(p.WorkloadType), pq.Array(p.HistoricalBottlenecks), p.Notes)
	return err
}

func (s *Store) AddBottleneck(sysID, bottleneck string) error {
	_, err := s.db.Exec(`
		UPDATE copilot_cluster_profiles
		SET historical_bottlenecks = array_append(historical_bottlenecks, $2),
		    updated_at = NOW()
		WHERE sys_id = $1
	`, sysID, bottleneck)
	return err
}

func (s *Store) Close() {
	s.db.Close()
}
