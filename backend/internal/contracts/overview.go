package contracts

import "time"

// CurrentUser is the authenticated subject's public organization view.
// Authentication and membership resolution are intentionally implemented later.
type CurrentUser struct {
	Subject       string                `json:"subject"`
	Organizations []OrganizationSummary `json:"organizations"`
}

type OrganizationSummary struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	Role OrganizationRole `json:"role"`
}

type OrganizationRole string

const (
	RoleOwner  OrganizationRole = "owner"
	RoleAdmin  OrganizationRole = "admin"
	RoleMember OrganizationRole = "member"
	RoleViewer OrganizationRole = "viewer"
)

type SiteListResponse struct {
	Items []SiteSummary `json:"items"`
}

type SiteSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type SiteOverview struct {
	Site      SiteSummary      `json:"site"`
	Readings  []ReadingSummary `json:"readings"`
	Devices   []DeviceSummary  `json:"devices"`
	UpdatedAt *time.Time       `json:"updatedAt"`
}

type ReadingSummary struct {
	Label      string     `json:"label"`
	Value      *float64   `json:"value"`
	Unit       string     `json:"unit"`
	MeasuredAt *time.Time `json:"measuredAt"`
}

type DeviceListResponse struct {
	Items []DeviceSummary `json:"items"`
}

type DeviceSummary struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Status *string `json:"status"`
}

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}
