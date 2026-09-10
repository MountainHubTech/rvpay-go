package auth

import (
	"context"
	"errors"
	"net/mail"
	"time"

	"github.com/google/uuid"

	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// User roles. The users table is the single source of truth; multiple users
// with the admin role are supported naturally, and user-role users must never
// authenticate to admin-protected endpoints. Hiding UI affordances is never
// authorization; every protected endpoint checks the role server-side.
const (
	UserRoleUser  = sqlc.UserRoleUSERROLEUSER
	UserRoleAdmin = sqlc.UserRoleUSERROLEADMIN
)

// Settings holds the tunable authentication parameters. They come from
// environment configuration, never hard-coded.
type Settings struct {
	// AccessTokenTTL is how long an access token is accepted (e.g. 1h).
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is how long the refresh token may rotate the access
	// token (e.g. 720h = 30 days).
	RefreshTokenTTL time.Duration
}

// Service implements the administrator authentication flow: sign-in, refresh
// rotation, sign-out and token validation. Administrator accounts are
// database-managed users; the service never seeds or selects an
// administrator from environment variables.
type Service struct {
	clientsgrpc.UnimplementedAuthServiceServer
	userRepo        repo.UserRepo
	accessTokenRepo repo.AccessTokenRepo
	settings        Settings
	logger          zerolog.Logger
}

// NewService constructs the authentication service.
func NewService(userRepo repo.UserRepo, accessTokenRepo repo.AccessTokenRepo, settings Settings, logger zerolog.Logger) *Service {
	return &Service{
		userRepo:        userRepo,
		accessTokenRepo: accessTokenRepo,
		settings:        settings,
		logger:          logger,
	}
}

// translateAuthError converts sentinel errors into gRPC status errors with
// deliberately generic messages (no user enumeration, no token details).
func translateAuthError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "invalid email or password")
	case errors.Is(err, ErrInvalidToken):
		return status.Error(codes.Unauthenticated, "invalid or expired token")
	case errors.Is(err, ErrEmailExists), errors.Is(err, repo.ErrDuplicate):
		return status.Error(codes.AlreadyExists, "user already exists")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// SignIn implements AuthService.SignIn. It verifies the argon2id password
// hash against the database-managed user, enforces the admin role, then
// issues a fresh opaque token pair; only SHA-256 hashes of the tokens are
// persisted. There is no special case for any configured or bootstrap
// administrator: authentication is entirely database/user based.
func (s *Service) SignIn(ctx context.Context, req *clientsgrpc.SignInRequest) (*clientsgrpc.SignInResponse, error) {
	if req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	user, err := s.userRepo.GetByEmail(ctx, req.GetEmail())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Same response shape and code as a wrong password: no user
			// enumeration. A dummy verification keeps timing roughly equal.
			VerifyPassword(req.GetPassword(), ArgonEncodedPrefix+"v=19$m=1024,t=1,p=1$c2FsdA$ZmFrZQ")
			return nil, translateAuthError(ErrInvalidCredentials)
		}
		return nil, translateAuthError(err)
	}
	if !VerifyPassword(req.GetPassword(), user.PasswordHash) {
		return nil, translateAuthError(ErrInvalidCredentials)
	}
	// USER_ROLE_USER accounts must not authenticate to the admin flow. The
	// failure is identical to a wrong password: still no enumeration.
	if user.UserRole != UserRoleAdmin {
		return nil, translateAuthError(ErrInvalidCredentials)
	}

	resp, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	s.logger.Info().Str("user_id", user.ID.String()).Msg("administrator signed in")
	return resp, nil
}

// RefreshToken implements AuthService.RefreshToken. The presented refresh
// token is matched by hash, rotated (old row deleted, new pair issued) and
// the user record's refresh hash is updated.
func (s *Service) RefreshToken(ctx context.Context, req *clientsgrpc.RefreshTokenRequest) (*clientsgrpc.RefreshTokenResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	hash := tokenHash(req.GetRefreshToken())
	record, err := s.accessTokenRepo.GetByRefreshTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, translateAuthError(ErrInvalidToken)
		}
		return nil, translateAuthError(err)
	}

	user, err := s.userRepo.GetByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, translateAuthError(ErrInvalidToken)
		}
		return nil, translateAuthError(err)
	}
	if user.UserRole != UserRoleAdmin {
		return nil, translateAuthError(ErrInvalidToken)
	}

	// Rotate: the presented refresh token is single-use. Deleting the row
	// atomically removes both the old access token and the refresh mapping.
	if err := s.accessTokenRepo.DeleteByTokenHash(ctx, record.TokenHash); err != nil {
		return nil, translateAuthError(err)
	}
	if err := s.userRepo.UpdateRefreshTokenHash(ctx, user.ID, ""); err != nil {
		return nil, translateAuthError(err)
	}

	resp, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	s.logger.Info().Str("user_id", user.ID.String()).Msg("administrator access token rotated")
	return &clientsgrpc.RefreshTokenResponse{
		User:                 resp.GetUser(),
		AccessToken:          resp.GetAccessToken(),
		RefreshToken:         resp.GetRefreshToken(),
		AccessTokenExpiresAt: resp.GetAccessTokenExpiresAt(),
	}, nil
}

// SignOut implements AuthService.SignOut. The presented access token is
// revoked immediately and the persisted session refresh mapping is cleared
// so the whole session dies. The raw token comes from the incoming gRPC
// metadata (Bearer scheme) via TokenFromContext.
func (s *Service) SignOut(ctx context.Context, req *clientsgrpc.SignOutRequest) (*clientsgrpc.SignOutResponse, error) {
	token := TokenFromContext(ctx)
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	hash := tokenHash(token)
	record, err := s.accessTokenRepo.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Already gone; sign-out is idempotent.
			return &clientsgrpc.SignOutResponse{}, nil
		}
		return nil, translateAuthError(err)
	}
	if err := s.accessTokenRepo.DeleteByTokenHash(ctx, hash); err != nil {
		return nil, translateAuthError(err)
	}
	if err := s.userRepo.ClearRefreshTokenHash(ctx, record.UserID); err != nil {
		return nil, translateAuthError(err)
	}

	s.logger.Info().Str("user_id", record.UserID.String()).Msg("administrator signed out")
	return &clientsgrpc.SignOutResponse{}, nil
}

// CreateUser implements AuthService.CreateUser. It is a gRPC-only
// provisioning method (no HTTP binding, no gateway route): the first
// administrator is created after deployment by calling it directly over
// gRPC. The plaintext password is hashed with Argon2id and never stored,
// logged, or returned. A unique-constraint failure surfaces as AlreadyExists
// without echoing the offending email beyond what the caller already sent.
func (s *Service) CreateUser(ctx context.Context, req *clientsgrpc.CreateUserRequest) (*clientsgrpc.CreateUserResponse, error) {
	if req.GetName() == "" || req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "name, email and password are required")
	}
	if err := validateEmail(req.GetEmail()); err != nil {
		return nil, err
	}
	if len(req.GetPassword()) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}
	role := sqlc.UserRole(req.GetUserRole())
	if role != UserRoleUser && role != UserRoleAdmin {
		return nil, status.Errorf(codes.InvalidArgument, "user_role must be %s or %s", UserRoleUser, UserRoleAdmin)
	}

	// Fail cleanly before hashing when the email is taken; the unique
	// constraint remains the authoritative backstop (ErrDuplicate maps to
	// the same AlreadyExists code, so a race cannot leak extra detail).
	if _, err := s.userRepo.GetByEmail(ctx, req.GetEmail()); err == nil {
		return nil, translateAuthError(ErrEmailExists)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, translateAuthError(err)
	}

	// Argon2id with a random per-user salt: the same plaintext never
	// produces the same stored hash for different users.
	passwordHash, err := HashPassword(req.GetPassword())
	if err != nil {
		return nil, translateAuthError(err)
	}

	// The insert is the transaction boundary: success is only reported
	// after the users row has been committed.
	user, err := s.userRepo.Create(ctx, req.GetName(), req.GetEmail(), passwordHash, role)
	if err != nil {
		return nil, translateAuthError(err)
	}

	s.logger.Info().Str("user_id", user.ID.String()).Str("user_role", string(user.UserRole)).Msg("user created")
	return &clientsgrpc.CreateUserResponse{
		User: &clientsgrpc.AdminUser{
			Id:       user.ID.String(),
			Name:     user.Name,
			Email:    user.Email,
			UserRole: string(user.UserRole),
		},
	}, nil
}

// UpdateUser implements AuthService.UpdateUser. Only name, email and
// password are mutable; the user's role is intentionally immutable. Empty
// request fields mean "unchanged" and never re-hash or overwrite existing
// values. Changing the password invalidates the persisted refresh mapping
// and deletes all access-token rows for the user so no pre-existing
// credential survives a credential change. No password or token material is
// ever logged or returned.
func (s *Service) UpdateUser(ctx context.Context, req *clientsgrpc.UpdateUserRequest) (*clientsgrpc.UpdateUserResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}
	userID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, translateAuthError(err)
	}

	name := user.Name
	if req.GetName() != "" {
		name = req.GetName()
	}
	email := user.Email
	if req.GetEmail() != "" {
		if err := validateEmail(req.GetEmail()); err != nil {
			return nil, err
		}
		email = req.GetEmail()
	}

	if email != user.Email {
		// Reject moving this account onto an email owned by another user;
		// the unique constraint backstops a concurrent change.
		other, err := s.userRepo.GetByEmail(ctx, email)
		if err == nil && other.ID != user.ID {
			return nil, translateAuthError(ErrEmailExists)
		} else if err != nil && !errors.Is(err, repo.ErrNotFound) {
			return nil, translateAuthError(err)
		}
	}

	if req.GetPassword() != "" {
		if len(req.GetPassword()) < 8 {
			return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
		}
		passwordHash, err := HashPassword(req.GetPassword())
		if err != nil {
			return nil, translateAuthError(err)
		}
		if _, err := s.userRepo.UpdatePasswordHash(ctx, user.ID, passwordHash); err != nil {
			return nil, translateAuthError(err)
		}
		// Refresh tokens are credentials: a password change must not leave
		// the old refresh token usable. Existing access tokens are deleted
		// with it so the caller must re-authenticate with the new password.
		if _, err := s.accessTokenRepo.DeleteByUserID(ctx, user.ID); err != nil {
			return nil, translateAuthError(err)
		}
		if err := s.userRepo.ClearRefreshTokenHash(ctx, user.ID); err != nil {
			return nil, translateAuthError(err)
		}
		s.logger.Info().Str("user_id", user.ID.String()).Msg("user password changed; sessions invalidated")
	}

	// Only persist name/email when something actually changed so the row
	// (and updated_at) is not rewritten unnecessarily.
	if name != user.Name || email != user.Email {
		user, err = s.userRepo.UpdateNameEmail(ctx, user.ID, name, email)
		if err != nil {
			return nil, translateAuthError(err)
		}
		s.logger.Info().Str("user_id", user.ID.String()).Msg("user updated")
	}

	return &clientsgrpc.UpdateUserResponse{
		User: &clientsgrpc.AdminUser{
			Id:       user.ID.String(),
			Name:     user.Name,
			Email:    user.Email,
			UserRole: string(user.UserRole),
		},
	}, nil
}

// userListTime renders a human-readable initiation time for the user list.
func userListTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006")
}

// ListUsers implements AuthService.ListUsers: a paginated, searchable,
// role-filtered list of database-managed users for the Admin Dashboard
// Settings team-management page. Password and token material are never
// returned — only safe identity fields.
func (s *Service) ListUsers(ctx context.Context, req *clientsgrpc.ListUsersRequest) (resp *clientsgrpc.ListUsersResponse, err error) {
	if req == nil {
		req = &clientsgrpc.ListUsersRequest{}
	}
	s.logger.Info().
		Str("endpoint", "/v1/public/clients/users").
		Str("method", "GET").
		Str("operation", "ListUsers").
		Str("search", req.GetSearch()).
		Str("role", req.GetRole()).
		Int("page", int(req.GetPage())).
		Int("page_size", int(req.GetPageSize())).
		Msg("dashboard API request received")

	defer func() {
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("endpoint", "/v1/public/clients/users").
				Str("operation", "ListUsers").
				Str("grpc_code", status.Code(err).String()).
				Msg("dashboard API request failed")
			return
		}
		s.logger.Info().
			Str("endpoint", "/v1/public/clients/users").
			Str("operation", "ListUsers").
			Str("grpc_code", "OK").
			Int("rows_returned", len(resp.GetRows())).
			Int64("total", resp.GetTotal()).
			Int("page", int(resp.GetPage())).
			Int("page_size", int(resp.GetPageSize())).
			Msg("dashboard API request completed")
	}()

	page := req.GetPage()
	if page < 1 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	rows, err := s.userRepo.ListUsers(ctx, req.GetSearch(), req.GetRole(), pageSize, offset)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListUsers").
			Str("repository", "UserRepo.ListUsers").
			Msg("could not list users")
		return nil, translateAuthError(err)
	}
	total, err := s.userRepo.CountUsers(ctx, req.GetSearch(), req.GetRole())
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListUsers").
			Str("repository", "UserRepo.CountUsers").
			Msg("could not count users")
		return nil, translateAuthError(err)
	}

	protoRows := make([]*clientsgrpc.UserRow, 0, len(rows))
	for _, row := range rows {
		status := "Inactive"
		if row.RefreshTokenHash != "" {
			status = "Active"
		}
		protoRows = append(protoRows, &clientsgrpc.UserRow{
			Id:         row.ID.String(),
			Name:       row.Name,
			Email:      row.Email,
			Role:       string(row.UserRole),
			Status:     status,
			DateJoined: userListTime(row.CreatedAt),
		})
	}

	return &clientsgrpc.ListUsersResponse{
		Rows:     protoRows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// validateEmail enforces a syntactically valid, non-empty email address.
func validateEmail(email string) error {
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return status.Error(codes.InvalidArgument, "a valid email address is required")
	}
	return nil
}

// ValidateAccessToken implements the INTERNAL AuthService.ValidateAccessToken
// RPC used by the other services' admin middleware.
func (s *Service) ValidateAccessToken(ctx context.Context, req *clientsgrpc.ValidateAccessTokenRequest) (*clientsgrpc.ValidateAccessTokenResponse, error) {
	if req.GetAccessToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	record, err := s.accessTokenRepo.GetByTokenHash(ctx, tokenHash(req.GetAccessToken()))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Unknown/expired tokens are simply invalid — not a transport
			// failure surfaced to the calling service.
			return &clientsgrpc.ValidateAccessTokenResponse{Valid: false}, nil
		}
		return nil, translateAuthError(err)
	}

	return &clientsgrpc.ValidateAccessTokenResponse{
		Valid:    true,
		UserId:   record.UserID.String(),
		UserRole: string(UserRoleAdmin),
	}, nil
}

// issueTokens creates a fresh access/refresh pair for the user. Only the
// SHA-256 hashes are persisted; the raw tokens are returned exactly once.
func (s *Service) issueTokens(ctx context.Context, user sqlc.User) (*clientsgrpc.SignInResponse, error) {
	accessToken, err := generateToken()
	if err != nil {
		return nil, translateAuthError(err)
	}
	refreshToken, err := generateToken()
	if err != nil {
		return nil, translateAuthError(err)
	}

	now := time.Now()
	expiresAt := now.Add(s.settings.AccessTokenTTL)
	refreshExpiresAt := now.Add(s.settings.RefreshTokenTTL)

	if _, err := s.accessTokenRepo.Create(ctx, user.ID, tokenHash(accessToken), tokenHash(refreshToken), expiresAt, refreshExpiresAt); err != nil {
		return nil, translateAuthError(err)
	}
	if err := s.userRepo.UpdateRefreshTokenHash(ctx, user.ID, tokenHash(refreshToken)); err != nil {
		return nil, translateAuthError(err)
	}

	return &clientsgrpc.SignInResponse{
		User: &clientsgrpc.AdminUser{
			Id:       user.ID.String(),
			Name:     user.Name,
			Email:    user.Email,
			UserRole: string(user.UserRole),
		},
		AccessToken:          accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: timestamppb.New(expiresAt),
	}, nil
}
