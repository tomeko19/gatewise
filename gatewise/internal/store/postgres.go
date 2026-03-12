// Package store provides PostgreSQL storage for Gatewise policies
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gatewise/gatewise/pkg/models"
	_ "github.com/lib/pq"
)

// PostgresStore implements policy storage using PostgreSQL
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresStore{db: db}

	// Initialize schema
	if err := store.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema
func (s *PostgresStore) initSchema(ctx context.Context) error {
	schema := `
	-- Policies table
	CREATE TABLE IF NOT EXISTS policies (
		id SERIAL PRIMARY KEY,
		api_version VARCHAR(50) NOT NULL,
		kind VARCHAR(50) NOT NULL,
		name VARCHAR(63) NOT NULL UNIQUE,
		namespace VARCHAR(63),
		labels JSONB DEFAULT '{}',
		annotations JSONB DEFAULT '{}',
		spec JSONB NOT NULL,
		status VARCHAR(20) DEFAULT 'Pending',
		status_message TEXT,
		last_applied_at TIMESTAMP,
		drift_detected BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_policies_kind ON policies(kind);
	CREATE INDEX IF NOT EXISTS idx_policies_namespace ON policies(namespace);
	CREATE INDEX IF NOT EXISTS idx_policies_status ON policies(status);
	CREATE INDEX IF NOT EXISTS idx_policies_labels ON policies USING GIN(labels);

	-- Audit log table
	CREATE TABLE IF NOT EXISTS audit_logs (
		id SERIAL PRIMARY KEY,
		policy_id INTEGER REFERENCES policies(id) ON DELETE SET NULL,
		policy_name VARCHAR(63) NOT NULL,
		action VARCHAR(50) NOT NULL,
		target_type VARCHAR(20),
		target_name VARCHAR(255),
		actor VARCHAR(255),
		details JSONB DEFAULT '{}',
		success BOOLEAN DEFAULT TRUE,
		error_message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create index for audit queries
	CREATE INDEX IF NOT EXISTS idx_audit_logs_policy_id ON audit_logs(policy_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

	-- JIT access grants table
	CREATE TABLE IF NOT EXISTS jit_grants (
		id SERIAL PRIMARY KEY,
		policy_id INTEGER REFERENCES policies(id) ON DELETE CASCADE,
		grantee VARCHAR(255) NOT NULL,
		granted_by VARCHAR(255),
		expires_at TIMESTAMP NOT NULL,
		renewals INTEGER DEFAULT 0,
		max_renewals INTEGER DEFAULT 0,
		status VARCHAR(20) DEFAULT 'Active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create index for JIT queries
	CREATE INDEX IF NOT EXISTS idx_jit_grants_expires_at ON jit_grants(expires_at);
	CREATE INDEX IF NOT EXISTS idx_jit_grants_status ON jit_grants(status);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// CreatePolicy stores a new policy
func (s *PostgresStore) CreatePolicy(ctx context.Context, policy *models.Policy) (int, error) {
	labelsJSON, _ := json.Marshal(policy.Metadata.Labels)
	annotationsJSON, _ := json.Marshal(policy.Metadata.Annotations)
	specJSON, _ := json.Marshal(policy.Spec)

	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO policies (api_version, kind, name, namespace, labels, annotations, spec)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		policy.APIVersion,
		policy.Kind,
		policy.Metadata.Name,
		policy.Metadata.Namespace,
		labelsJSON,
		annotationsJSON,
		specJSON,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create policy: %w", err)
	}

	return id, nil
}

// GetPolicy retrieves a policy by name
func (s *PostgresStore) GetPolicy(ctx context.Context, name string) (*models.Policy, error) {
	var (
		id            int
		apiVersion    string
		kind          string
		namespace     sql.NullString
		labelsJSON    []byte
		annotationsJSON []byte
		specJSON      []byte
		createdAt     time.Time
		updatedAt     time.Time
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, api_version, kind, name, namespace, labels, annotations, spec, created_at, updated_at
		FROM policies WHERE name = $1
	`, name).Scan(&id, &apiVersion, &kind, &name, &namespace, &labelsJSON, &annotationsJSON, &specJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	policy := &models.Policy{
		APIVersion: apiVersion,
		Kind:       models.PolicyKind(kind),
		Metadata: models.PolicyMetadata{
			Name:      name,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	if namespace.Valid {
		policy.Metadata.Namespace = namespace.String
	}

	json.Unmarshal(labelsJSON, &policy.Metadata.Labels)
	json.Unmarshal(annotationsJSON, &policy.Metadata.Annotations)
	json.Unmarshal(specJSON, &policy.Spec)

	return policy, nil
}

// ListPolicies retrieves all policies with optional filtering
func (s *PostgresStore) ListPolicies(ctx context.Context, kind string, namespace string) ([]*models.Policy, error) {
	query := `SELECT api_version, kind, name, namespace, labels, annotations, spec, created_at, updated_at FROM policies WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if kind != "" {
		query += fmt.Sprintf(" AND kind = $%d", argIndex)
		args = append(args, kind)
		argIndex++
	}

	if namespace != "" {
		query += fmt.Sprintf(" AND namespace = $%d", argIndex)
		args = append(args, namespace)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	defer rows.Close()

	var policies []*models.Policy
	for rows.Next() {
		var (
			apiVersion      string
			kindStr         string
			name            string
			namespaceVal    sql.NullString
			labelsJSON      []byte
			annotationsJSON []byte
			specJSON        []byte
			createdAt       time.Time
			updatedAt       time.Time
		)

		if err := rows.Scan(&apiVersion, &kindStr, &name, &namespaceVal, &labelsJSON, &annotationsJSON, &specJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		policy := &models.Policy{
			APIVersion: apiVersion,
			Kind:       models.PolicyKind(kindStr),
			Metadata: models.PolicyMetadata{
				Name:      name,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		}

		if namespaceVal.Valid {
			policy.Metadata.Namespace = namespaceVal.String
		}

		json.Unmarshal(labelsJSON, &policy.Metadata.Labels)
		json.Unmarshal(annotationsJSON, &policy.Metadata.Annotations)
		json.Unmarshal(specJSON, &policy.Spec)

		policies = append(policies, policy)
	}

	return policies, nil
}

// UpdatePolicyStatus updates the status of a policy
func (s *PostgresStore) UpdatePolicyStatus(ctx context.Context, name string, status models.PolicyState, message string) error {
	statusStr := string(status)
	_, err := s.db.ExecContext(ctx, `
		UPDATE policies 
		SET status = $1, status_message = $2, updated_at = CURRENT_TIMESTAMP,
		    last_applied_at = CASE WHEN $3 = 'Applied' THEN CURRENT_TIMESTAMP ELSE last_applied_at END
		WHERE name = $4
	`, statusStr, message, statusStr, name)

	if err != nil {
		return fmt.Errorf("failed to update policy status: %w", err)
	}
	return nil
}

// DeletePolicy removes a policy by name
func (s *PostgresStore) DeletePolicy(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM policies WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}
	return nil
}

// CreateAuditLog creates an audit log entry
func (s *PostgresStore) CreateAuditLog(ctx context.Context, log *AuditLogEntry) error {
	detailsJSON, _ := json.Marshal(log.Details)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (policy_id, policy_name, action, target_type, target_name, actor, details, success, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, log.PolicyID, log.PolicyName, log.Action, log.TargetType, log.TargetName, log.Actor, detailsJSON, log.Success, log.ErrorMessage)

	return err
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	PolicyID     *int
	PolicyName   string
	Action       string
	TargetType   string
	TargetName   string
	Actor        string
	Details      map[string]interface{}
	Success      bool
	ErrorMessage string
}
