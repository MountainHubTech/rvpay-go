package auth

import (
	"context"
	"errors"
	"time"

	"github.com/I-Frostbyte/rvpay-go/clients/db/repo"
	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	clientsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserRole is the single role the backend enforces for authorization. Hiding
// UI affordances is never authorization; every protected endpoint checks it
// server-side.
const UserRole = "admin"

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
// rotation, sign-out, token validation and the env-seeded bootstrap admin.
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
	case errors.Is(err, ErrEmailExists):
		return status.Error(codes.AlreadyExists, "administrator already exists")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// SignIn implements AuthService.SignIn. It verifies the argon2id password
// hash, then issues a fresh opaque token pair; only SHA-256 hashes of the
// tokens are persisted.
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
			return nil, ErrInvalidCredentials
		}
		return nil, translateAuthError(err)
	}
	if !VerifyPassword(req.GetPassword(), user.PasswordHash) {
		return nil, ErrInvalidCredentials
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
			return nil, ErrInvalidToken
		}
		return nil, translateAuthError(err)
	}

	user, err := s.userRepo.GetByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, translateAuthError(err)
	}
	if user.UserRole != UserRole {
		return nil, ErrInvalidToken
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
		UserRole: UserRole,
	}, nil
}

// BootstrapAdmin seeds the first administrator from environment
// configuration when no user exists yet. It is idempotent: it only acts on
// an empty users table and never overwrites an existing administrator.
func (s *Service) BootstrapAdmin(ctx context.Context, name, email, password string) error {
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if name == "" || email == "" || password == "" {
		s.logger.Warn().Msg("no administrators exist and ADMIN_NAME/ADMIN_EMAIL/ADMIN_PASSWORD are not fully configured; admin sign-in is unavailable until configured")
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	user, err := s.userRepo.Create(ctx, name, email, hash, UserRole)
	if err != nil {
		return err
	}
	s.logger.Info().Str("user_id", user.ID.String()).Msg("bootstrap administrator created")
	return nil
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
			UserRole: user.UserRole,
		},
		AccessToken:          accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: timestamppb.New(expiresAt),
	}, nil
}
