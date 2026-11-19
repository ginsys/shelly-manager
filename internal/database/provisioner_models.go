package database

import "time"

// Table names of the provisioner registry. The MySQL step in migrations.go names them too.
const (
	provisionerAgentsTable = "provisioner_agents"
	provisioningTasksTable = "provisioning_tasks"
)

// ProvisionerAgent is one registered provisioning agent. The manager keeps the SHA-256 of the agent's
// token and never the token itself (ProvisionerStore, HashAgentToken).
//
// The JSON columns (capabilities, metadata, and a task's config) name no column type: GORM gives a
// serializer field its string type, which is TEXT on SQLite and PostgreSQL and LONGTEXT on MySQL, so
// client input of up to the 10 MiB request limit fits on all three. A type:text tag would be TEXT
// (64 KiB) on MySQL, and a MEDIUMTEXT set by hand would be narrowed back by the next AutoMigrate.
type ProvisionerAgent struct {
	ID           uint              `json:"-" gorm:"primaryKey"`
	AgentID      string            `json:"id" gorm:"size:191;not null;uniqueIndex"`
	TokenHash    string            `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Hostname     string            `json:"hostname"`
	IP           string            `json:"ip"`
	Version      string            `json:"version,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty" gorm:"serializer:json"`
	Status       string            `json:"status" gorm:"size:191"`
	Metadata     map[string]string `json:"metadata,omitempty" gorm:"serializer:json"`
	LastSeen     time.Time         `json:"last_seen" gorm:"index"`
	RegisteredAt time.Time         `json:"registered_at"`
	CreatedAt    time.Time         `json:"-"`
	UpdatedAt    time.Time         `json:"-"`
}

// TableName names the table, so the MySQL step and the model cannot drift apart.
func (ProvisionerAgent) TableName() string { return provisionerAgentsTable }

// ProvisioningTask is one task for a provisioning agent. An empty AgentID means nobody has claimed it.
type ProvisioningTask struct {
	ID         uint                   `json:"-" gorm:"primaryKey"`
	TaskID     string                 `json:"id" gorm:"size:191;not null;uniqueIndex"`
	Type       string                 `json:"type" gorm:"size:191;not null"`
	DeviceMAC  string                 `json:"device_mac,omitempty"`
	TargetSSID string                 `json:"target_ssid,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty" gorm:"serializer:json"`
	Status     string                 `json:"status" gorm:"size:191;not null;index"`
	AgentID    string                 `json:"agent_id,omitempty" gorm:"size:191;not null;index"`
	Priority   int                    `json:"priority,omitempty"`
	CreatedAt  time.Time              `json:"created_at" gorm:"index"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// TableName names the table, see ProvisionerAgent.TableName.
func (ProvisioningTask) TableName() string { return provisioningTasksTable }
