// Command membership performs administrative membership operations.
//
// It is the only supported way to create organizations and grant or revoke
// memberships: it uses the administrative database credential, whose role can
// execute the audited admin functions and nothing else.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/internal/storage/postgres"
)

var (
	ErrUnknownCommand            = errors.New("unknown command")
	ErrInvalidArguments          = errors.New("invalid arguments")
	ErrInvalidRole               = errors.New("invalid membership role")
	ErrMissingAdminDatabaseURL   = errors.New("ADMIN_DATABASE_URL is required")
	ErrMissingSystemAdminSubject = errors.New("SYSTEM_ADMIN_SUBJECT is required")
)

const (
	actionCreateOrganization = "create-organization"
	actionGrant              = "grant"
	actionRevoke             = "revoke"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

type adminAction struct {
	kind           string
	organizationID string
	subject        string
	role           contracts.OrganizationRole
	name           string
}

type adminConfig struct {
	adminDatabaseURL   string
	systemAdminSubject string
}

func parseAction(args []string) (adminAction, error) {
	if len(args) == 0 {
		return adminAction{}, fmt.Errorf("%w: expected a command", ErrInvalidArguments)
	}

	switch args[0] {
	case actionCreateOrganization:
		if len(args) != 3 {
			return adminAction{}, fmt.Errorf("%w: usage: %s <organization-id> <name>", ErrInvalidArguments, actionCreateOrganization)
		}
		return adminAction{
			kind:           actionCreateOrganization,
			organizationID: strings.TrimSpace(args[1]),
			name:           strings.TrimSpace(args[2]),
		}, nil
	case actionGrant:
		if len(args) != 4 {
			return adminAction{}, fmt.Errorf("%w: usage: %s <organization-id> <subject> <role>", ErrInvalidArguments, actionGrant)
		}
		role := contracts.OrganizationRole(strings.TrimSpace(args[3]))
		switch role {
		case contracts.RoleOwner, contracts.RoleAdmin, contracts.RoleMember, contracts.RoleViewer:
		default:
			return adminAction{}, fmt.Errorf("%w: %q", ErrInvalidRole, args[3])
		}
		return adminAction{
			kind:           actionGrant,
			organizationID: strings.TrimSpace(args[1]),
			subject:        strings.TrimSpace(args[2]),
			role:           role,
		}, nil
	case actionRevoke:
		if len(args) != 3 {
			return adminAction{}, fmt.Errorf("%w: usage: %s <organization-id> <subject>", ErrInvalidArguments, actionRevoke)
		}
		return adminAction{
			kind:           actionRevoke,
			organizationID: strings.TrimSpace(args[1]),
			subject:        strings.TrimSpace(args[2]),
		}, nil
	default:
		return adminAction{}, fmt.Errorf("%w: %q", ErrUnknownCommand, args[0])
	}
}

func loadAdminConfig(getenv func(string) string) (adminConfig, error) {
	if getenv == nil {
		return adminConfig{}, errors.New("environment reader is required")
	}
	adminDatabaseURL := strings.TrimSpace(getenv("ADMIN_DATABASE_URL"))
	if adminDatabaseURL == "" {
		return adminConfig{}, ErrMissingAdminDatabaseURL
	}
	systemAdminSubject := strings.TrimSpace(getenv("SYSTEM_ADMIN_SUBJECT"))
	if systemAdminSubject == "" {
		return adminConfig{}, ErrMissingSystemAdminSubject
	}
	return adminConfig{
		adminDatabaseURL:   adminDatabaseURL,
		systemAdminSubject: systemAdminSubject,
	}, nil
}

func run(args []string) error {
	action, err := parseAction(args)
	if err != nil {
		return err
	}
	config, err := loadAdminConfig(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, config.adminDatabaseURL)
	if err != nil {
		return fmt.Errorf("connect admin database: %w", postgres.RedactConnectionError(err, config.adminDatabaseURL))
	}
	defer pool.Close()

	db := postgres.NewDB(pool)
	principal := authz.Principal{Subject: config.systemAdminSubject}

	switch action.kind {
	case actionCreateOrganization:
		service := authz.NewAdminOrganizationService(
			config.systemAdminSubject,
			postgres.NewOrganizationWriter(db, config.systemAdminSubject),
		)
		if err := service.Create(ctx, principal, action.organizationID, action.name); err != nil {
			return adminError("create organization", err, config.adminDatabaseURL)
		}
	case actionGrant:
		service := authz.NewAdminMembershipService(
			config.systemAdminSubject,
			postgres.NewMembershipWriter(db, config.systemAdminSubject),
		)
		if err := service.Grant(ctx, principal, action.organizationID, action.subject, action.role); err != nil {
			return adminError("grant membership", err, config.adminDatabaseURL)
		}
	case actionRevoke:
		service := authz.NewAdminMembershipService(
			config.systemAdminSubject,
			postgres.NewMembershipWriter(db, config.systemAdminSubject),
		)
		if err := service.Revoke(ctx, principal, action.organizationID, action.subject); err != nil {
			return adminError("revoke membership", err, config.adminDatabaseURL)
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownCommand, action.kind)
	}

	log.Printf("%s completed for organization %q", action.kind, action.organizationID)
	return nil
}

// adminError wraps an administrative failure without letting the password of
// the administrative credential reach the terminal or the log.
func adminError(operation string, err error, adminDatabaseURL string) error {
	return fmt.Errorf("%s: %w", operation, postgres.RedactConnectionError(err, adminDatabaseURL))
}
