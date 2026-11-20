package authz

import (
	"context"
	"errors"
	"strings"

	"energyhub-universal/backend/internal/contracts"
)

var (
	ErrAdminNotConfigured            = errors.New("system admin is not configured")
	ErrAdminForbidden                = errors.New("principal is not the system admin")
	ErrInvalidMembershipTarget       = errors.New("membership target is invalid")
	ErrInvalidOrganizationTarget     = errors.New("organization target is invalid")
	ErrMembershipWriterUnavailable   = errors.New("membership writer unavailable")
	ErrOrganizationWriterUnavailable = errors.New("organization writer unavailable")
	ErrInvalidSiteTarget             = errors.New("site target is invalid")
	ErrSiteWriterUnavailable         = errors.New("site writer unavailable")
)

// MembershipWriter is the administrative write port. The adminSubject is the
// already verified principal and must never come from a request body.
type MembershipWriter interface {
	Grant(ctx context.Context, adminSubject, organizationID, subject string, role contracts.OrganizationRole) error
	Revoke(ctx context.Context, adminSubject, organizationID, subject string) error
}

// AdminAuthorizer compares a verified principal with the single configured
// system administrator. An empty configuration fails closed.
type AdminAuthorizer struct {
	subject string
}

func NewAdminAuthorizer(subject string) *AdminAuthorizer {
	return &AdminAuthorizer{subject: strings.TrimSpace(subject)}
}

func (a *AdminAuthorizer) Authorize(principal Principal) error {
	if a == nil || a.subject == "" {
		return ErrAdminNotConfigured
	}
	subject := strings.TrimSpace(principal.Subject)
	if subject == "" {
		return ErrUnauthenticated
	}
	if subject != a.subject {
		return ErrAdminForbidden
	}
	return nil
}

// AdminMembershipService is the only path allowed to call MembershipWriter.
// Authorization happens before validation or storage access.
type AdminMembershipService struct {
	admin  *AdminAuthorizer
	writer MembershipWriter
}

func NewAdminMembershipService(adminSubject string, writer MembershipWriter) *AdminMembershipService {
	return &AdminMembershipService{
		admin:  NewAdminAuthorizer(adminSubject),
		writer: writer,
	}
}

func (s *AdminMembershipService) Grant(
	ctx context.Context,
	principal Principal,
	organizationID string,
	subject string,
	role contracts.OrganizationRole,
) error {
	if err := s.authorize(principal); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	subject = strings.TrimSpace(subject)
	if organizationID == "" || subject == "" || !validRole(role) {
		return ErrInvalidMembershipTarget
	}
	return s.writer.Grant(ctx, strings.TrimSpace(principal.Subject), organizationID, subject, role)
}

func (s *AdminMembershipService) Revoke(
	ctx context.Context,
	principal Principal,
	organizationID string,
	subject string,
) error {
	if err := s.authorize(principal); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	subject = strings.TrimSpace(subject)
	if organizationID == "" || subject == "" {
		return ErrInvalidMembershipTarget
	}
	return s.writer.Revoke(ctx, strings.TrimSpace(principal.Subject), organizationID, subject)
}

func (s *AdminMembershipService) authorize(principal Principal) error {
	if s == nil || s.admin == nil {
		return ErrAdminNotConfigured
	}
	if err := s.admin.Authorize(principal); err != nil {
		return err
	}
	if s.writer == nil {
		return ErrMembershipWriterUnavailable
	}
	return nil
}

// OrganizationWriter is the administrative write port for organizations.
type OrganizationWriter interface {
	Create(ctx context.Context, adminSubject, organizationID, name string) error
}

// AdminOrganizationService is the only path allowed to create organizations.
type AdminOrganizationService struct {
	admin  *AdminAuthorizer
	writer OrganizationWriter
}

func NewAdminOrganizationService(adminSubject string, writer OrganizationWriter) *AdminOrganizationService {
	return &AdminOrganizationService{
		admin:  NewAdminAuthorizer(adminSubject),
		writer: writer,
	}
}

// Create authorizes the principal before validating or touching storage.
func (s *AdminOrganizationService) Create(
	ctx context.Context,
	principal Principal,
	organizationID string,
	name string,
) error {
	if err := s.authorize(principal); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	name = strings.TrimSpace(name)
	if organizationID == "" || name == "" {
		return ErrInvalidOrganizationTarget
	}
	return s.writer.Create(ctx, strings.TrimSpace(principal.Subject), organizationID, name)
}

func (s *AdminOrganizationService) authorize(principal Principal) error {
	if s == nil || s.admin == nil {
		return ErrAdminNotConfigured
	}
	if err := s.admin.Authorize(principal); err != nil {
		return err
	}
	if s.writer == nil {
		return ErrOrganizationWriterUnavailable
	}
	return nil
}

// SiteWriter is the administrative write port for sites and devices.
type SiteWriter interface {
	CreateSite(ctx context.Context, adminSubject, organizationID, siteID, name, timezone string) error
	CreateDevice(ctx context.Context, adminSubject, organizationID, siteID, deviceID, name, kind string) error
}

// AdminSiteService is the only path allowed to create sites and devices.
type AdminSiteService struct {
	admin  *AdminAuthorizer
	writer SiteWriter
}

func NewAdminSiteService(adminSubject string, writer SiteWriter) *AdminSiteService {
	return &AdminSiteService{
		admin:  NewAdminAuthorizer(adminSubject),
		writer: writer,
	}
}

// CreateSite authorizes the principal before validating or touching storage.
func (s *AdminSiteService) CreateSite(
	ctx context.Context,
	principal Principal,
	organizationID string,
	siteID string,
	name string,
	timezone string,
) error {
	if err := s.authorize(principal); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	siteID = strings.TrimSpace(siteID)
	name = strings.TrimSpace(name)
	timezone = strings.TrimSpace(timezone)
	if organizationID == "" || siteID == "" || name == "" || timezone == "" {
		return ErrInvalidSiteTarget
	}
	return s.writer.CreateSite(ctx, strings.TrimSpace(principal.Subject), organizationID, siteID, name, timezone)
}

// CreateDevice authorizes the principal before validating or touching storage.
func (s *AdminSiteService) CreateDevice(
	ctx context.Context,
	principal Principal,
	organizationID string,
	siteID string,
	deviceID string,
	name string,
	kind string,
) error {
	if err := s.authorize(principal); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	siteID = strings.TrimSpace(siteID)
	deviceID = strings.TrimSpace(deviceID)
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if organizationID == "" || siteID == "" || deviceID == "" || name == "" || kind == "" {
		return ErrInvalidSiteTarget
	}
	return s.writer.CreateDevice(ctx, strings.TrimSpace(principal.Subject), organizationID, siteID, deviceID, name, kind)
}

func (s *AdminSiteService) authorize(principal Principal) error {
	if s == nil || s.admin == nil {
		return ErrAdminNotConfigured
	}
	if err := s.admin.Authorize(principal); err != nil {
		return err
	}
	if s.writer == nil {
		return ErrSiteWriterUnavailable
	}
	return nil
}

func validRole(role contracts.OrganizationRole) bool {
	switch role {
	case contracts.RoleOwner, contracts.RoleAdmin, contracts.RoleMember, contracts.RoleViewer:
		return true
	default:
		return false
	}
}
