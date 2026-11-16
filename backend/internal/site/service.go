package site

import (
	"context"
	"errors"
	"strings"

	"energyhub-universal/backend/internal/contracts"
)

var (
	ErrInvalidScope      = errors.New("invalid organization scope")
	ErrInvalidSiteID     = errors.New("invalid site id")
	ErrReaderUnavailable = errors.New("site reader unavailable")
)

// Scope is produced by a future authenticated request boundary.
// It must never be populated from an untrusted tenant header.
type Scope struct {
	OrganizationID string
	Subject        string
}

func (s Scope) valid() bool {
	return strings.TrimSpace(s.OrganizationID) != "" && strings.TrimSpace(s.Subject) != ""
}

// Reader is the tenant-aware persistence port for site data.
// Every operation requires organizationID so implementations cannot omit
// the organization predicate from their queries.
type Reader interface {
	ListSites(ctx context.Context, organizationID string) ([]contracts.SiteSummary, error)
	GetOverview(ctx context.Context, organizationID, siteID string) (contracts.SiteOverview, error)
	ListDevices(ctx context.Context, organizationID, siteID string) ([]contracts.DeviceSummary, error)
}

type Service struct {
	reader Reader
}

func NewService(reader Reader) *Service {
	return &Service{reader: reader}
}

func (s *Service) ListSites(ctx context.Context, scope Scope) (contracts.SiteListResponse, error) {
	if err := s.validate(scope); err != nil {
		return contracts.SiteListResponse{}, err
	}

	items, err := s.reader.ListSites(ctx, scope.OrganizationID)
	if err != nil {
		return contracts.SiteListResponse{}, err
	}
	return contracts.SiteListResponse{Items: items}, nil
}

func (s *Service) GetOverview(ctx context.Context, scope Scope, siteID string) (contracts.SiteOverview, error) {
	if err := s.validate(scope); err != nil {
		return contracts.SiteOverview{}, err
	}
	if strings.TrimSpace(siteID) == "" {
		return contracts.SiteOverview{}, ErrInvalidSiteID
	}

	return s.reader.GetOverview(ctx, scope.OrganizationID, siteID)
}

func (s *Service) ListDevices(ctx context.Context, scope Scope, siteID string) (contracts.DeviceListResponse, error) {
	if err := s.validate(scope); err != nil {
		return contracts.DeviceListResponse{}, err
	}
	if strings.TrimSpace(siteID) == "" {
		return contracts.DeviceListResponse{}, ErrInvalidSiteID
	}

	items, err := s.reader.ListDevices(ctx, scope.OrganizationID, siteID)
	if err != nil {
		return contracts.DeviceListResponse{}, err
	}
	return contracts.DeviceListResponse{Items: items}, nil
}

func (s *Service) validate(scope Scope) error {
	if s == nil || s.reader == nil {
		return ErrReaderUnavailable
	}
	if !scope.valid() {
		return ErrInvalidScope
	}
	return nil
}
