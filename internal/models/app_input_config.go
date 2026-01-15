package models

import (
	"encoding/json"
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// AppInputConfig stores the local configuration for which app inputs are customer-facing.
// This allows vendors to select which inputs customers can configure, independent of the Nuon API's source field.
// It also stores custom ordering for groups and inputs within groups.
type AppInputConfig struct {
	ID                 string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID        string         `gorm:"index;uniqueIndex:idx_workspace_app" json:"workspace_id"` // NEW: Workspace ownership (nullable for migration)
	OrgID              string         `gorm:"not null" json:"org_id"`                                  // FK to NuonOrg.ID (kept as reference)
	AppID              string         `gorm:"not null;uniqueIndex:idx_workspace_app" json:"app_id"`    // Nuon app ID
	CustomerInputNames string         `gorm:"type:text" json:"customer_input_names,omitempty"`         // JSON array of input names that are customer-facing
	GroupOrder         string         `gorm:"type:text" json:"group_order,omitempty"`                  // JSON array of group names in display order
	InputOrder         string         `gorm:"type:text" json:"input_order,omitempty"`                  // JSON object: {"group_name": ["input1", "input2"]}
	CollapsedGroups    string         `gorm:"type:text" json:"collapsed_groups,omitempty"`             // JSON array of group names that are collapsed by default
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"` // NEW
	NuonOrg   NuonOrg   `gorm:"foreignKey:OrgID;references:ID" json:"nuon_org,omitempty"`
}

// TableName ensures GORM creates a proper table name
func (AppInputConfig) TableName() string {
	return "app_input_configs"
}

func (aic *AppInputConfig) BeforeCreate(tx *gorm.DB) error {
	if aic.ID == "" {
		aic.ID = shortid.NewAppInputConfigID()
	}
	return nil
}

// GetCustomerInputNames returns the customer input names as a slice
func (aic *AppInputConfig) GetCustomerInputNames() []string {
	if aic.CustomerInputNames == "" {
		return []string{}
	}
	var names []string
	if err := json.Unmarshal([]byte(aic.CustomerInputNames), &names); err != nil {
		return []string{}
	}
	return names
}

// SetCustomerInputNames stores the customer input names as JSON
func (aic *AppInputConfig) SetCustomerInputNames(names []string) error {
	if names == nil || len(names) == 0 {
		aic.CustomerInputNames = ""
		return nil
	}
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	aic.CustomerInputNames = string(data)
	return nil
}

// IsCustomerInput checks if the given input name is customer-facing
func (aic *AppInputConfig) IsCustomerInput(name string) bool {
	names := aic.GetCustomerInputNames()
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// GetGroupOrder returns the group ordering as a slice
func (aic *AppInputConfig) GetGroupOrder() []string {
	if aic.GroupOrder == "" {
		return []string{}
	}
	var order []string
	if err := json.Unmarshal([]byte(aic.GroupOrder), &order); err != nil {
		return []string{}
	}
	return order
}

// SetGroupOrder stores the group ordering as JSON
func (aic *AppInputConfig) SetGroupOrder(order []string) error {
	if order == nil || len(order) == 0 {
		aic.GroupOrder = ""
		return nil
	}
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}
	aic.GroupOrder = string(data)
	return nil
}

// GetInputOrder returns the input ordering per group as a map
func (aic *AppInputConfig) GetInputOrder() map[string][]string {
	if aic.InputOrder == "" {
		return map[string][]string{}
	}
	var order map[string][]string
	if err := json.Unmarshal([]byte(aic.InputOrder), &order); err != nil {
		return map[string][]string{}
	}
	return order
}

// SetInputOrder stores the input ordering per group as JSON
func (aic *AppInputConfig) SetInputOrder(order map[string][]string) error {
	if order == nil || len(order) == 0 {
		aic.InputOrder = ""
		return nil
	}
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}
	aic.InputOrder = string(data)
	return nil
}

// GetCollapsedGroups returns the collapsed group names as a slice
func (aic *AppInputConfig) GetCollapsedGroups() []string {
	if aic.CollapsedGroups == "" {
		return []string{}
	}
	var names []string
	if err := json.Unmarshal([]byte(aic.CollapsedGroups), &names); err != nil {
		return []string{}
	}
	return names
}

// SetCollapsedGroups stores the collapsed group names as JSON
func (aic *AppInputConfig) SetCollapsedGroups(names []string) error {
	if names == nil || len(names) == 0 {
		aic.CollapsedGroups = ""
		return nil
	}
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	aic.CollapsedGroups = string(data)
	return nil
}

// IsGroupCollapsed checks if the given group name is collapsed by default
func (aic *AppInputConfig) IsGroupCollapsed(groupName string) bool {
	names := aic.GetCollapsedGroups()
	for _, n := range names {
		if n == groupName {
			return true
		}
	}
	return false
}
