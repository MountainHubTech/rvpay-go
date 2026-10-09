package auth

// Focused tests for the database-managed user authentication flow:
// multiple admins, role enforcement, no enumeration, token isolation,
// expiry, single-use refresh rotation and sign-out. No test calls the real
// database; the fakes mirror the repository/SQL semantics.

import (
	"context"
	"strings"
	"testing"
	"time"

	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newTestService(users *fakeUserRepo, tokens *fakeAccessTokenRepo) *Service {
	return NewService(users, tokens, Settings{
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}, zerolog.Nop())
}

func signIn(t *testing.T, svc *Service, email, password string) (*clientsgrpc.SignInResponse, error) {
	t.Helper()
	return svc.SignIn(context.Background(), &clientsgrpc.SignInRequest{Email: email, Password: password})
}

// Multiple admins: two independently seeded administrators both authenticate
// and each receives their own distinct token pair.
func TestMultipleAdminsAuthenticate(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)

	if _, err := users.seed("Admin A", "admin-a@example.com", "password-a", UserRoleAdmin); err != nil {
		t.Fatalf("seed admin a: %v", err)
	}
	if _, err := users.seed("Admin B", "admin-b@example.com", "password-b", UserRoleAdmin); err != nil {
		t.Fatalf("seed admin b: %v", err)
	}

	respA, err := signIn(t, svc, "admin-a@example.com", "password-a")
	if err != nil {
		t.Fatalf("admin a sign-in: %v", err)
	}
	respB, err := signIn(t, svc, "admin-b@example.com", "password-b")
	if err != nil {
		t.Fatalf("admin b sign-in: %v", err)
	}

	if respA.GetUser().GetUserRole() != string(UserRoleAdmin) {
		t.Errorf("admin a role = %q, want %q", respA.GetUser().GetUserRole(), string(UserRoleAdmin))
	}
	if respA.GetAccessToken() == respB.GetAccessToken() {
		t.Error("two administrators must never receive the same access token")
	}
	if respA.GetUser().GetId() == respB.GetUser().GetId() {
		t.Error("two distinct administrators must have distinct user ids")
	}
}

// A USER_ROLE_USER account must not authenticate to the admin flow, and the
// failure must be indistinguishable from a wrong password (no enumeration).
func TestUserRoleUserCannotAuthenticate(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("David", "user@example.com", "user-password", UserRoleUser); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, err := signIn(t, svc, "user@example.com", "user-password")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("user-role sign-in code = %v, want Unauthenticated", status.Code(err))
	}

	_, adminErr := signIn(t, svc, "no-such-admin@example.com", "whatever")
	if status.Code(err) != status.Code(adminErr) || status.Convert(err).Message() != status.Convert(adminErr).Message() {
		t.Errorf("user-role rejection must look identical to unknown-email rejection; got %q vs %q",
			status.Convert(err).Message(), status.Convert(adminErr).Message())
	}
}

func TestInvalidPasswordFails(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("Admin A", "admin-a@example.com", "correct-password", UserRoleAdmin); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := signIn(t, svc, "admin-a@example.com", "wrong-password"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("invalid password code = %v, want Unauthenticated", status.Code(err))
	}
}

// Unknown email must fail with exactly the same code and message as a wrong
// password: no information about whether the account exists.
func TestUnknownEmailNoEnumeration(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("Admin A", "admin-a@example.com", "correct-password", UserRoleAdmin); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, wrongPasswordErr := signIn(t, svc, "admin-a@example.com", "wrong-password")
	_, unknownEmailErr := signIn(t, svc, "ghost@example.com", "whatever")

	if status.Code(wrongPasswordErr) != status.Code(unknownEmailErr) ||
		status.Convert(wrongPasswordErr).Message() != status.Convert(unknownEmailErr).Message() {
		t.Errorf("unknown-email error must match wrong-password error exactly")
	}
}

// Token isolation: each access token validates only as its own user; a
// token can never surface another administrator's identity.
func TestTokenIsolation(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	adminA, _ := users.seed("Admin A", "admin-a@example.com", "password-a", UserRoleAdmin)
	adminB, _ := users.seed("Admin B", "admin-b@example.com", "password-b", UserRoleAdmin)

	respA, err := signIn(t, svc, "admin-a@example.com", "password-a")
	if err != nil {
		t.Fatalf("admin a sign-in: %v", err)
	}
	respB, err := signIn(t, svc, "admin-b@example.com", "password-b")
	if err != nil {
		t.Fatalf("admin b sign-in: %v", err)
	}

	validate := func(token string) *clientsgrpc.ValidateAccessTokenResponse {
		resp, err := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: token})
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		return resp
	}

	validA := validate(respA.GetAccessToken())
	if !validA.GetValid() || validA.GetUserId() != adminA.ID.String() {
		t.Errorf("token A must authenticate as admin A, got valid=%v user=%q", validA.GetValid(), validA.GetUserId())
	}
	validB := validate(respB.GetAccessToken())
	if !validB.GetValid() || validB.GetUserId() != adminB.ID.String() {
		t.Errorf("token B must authenticate as admin B, got valid=%v user=%q", validB.GetValid(), validB.GetUserId())
	}
	if validA.GetUserId() == validB.GetUserId() {
		t.Error("distinct administrators must never share a token identity")
	}
}

// Expired access tokens are rejected. The fake mirrors the SQL expiry
// filter, so a token whose expiry is in the past simply does not exist.
func TestExpiredTokenRejected(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("Admin A", "admin-a@example.com", "password-a", UserRoleAdmin); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resp, err := signIn(t, svc, "admin-a@example.com", "password-a")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}

	hash := tokenHash(resp.GetAccessToken())
	tokens.mu.Lock()
	row := tokens.rows[hash]
	row.expiresAt = time.Now().Add(-time.Minute)
	tokens.rows[hash] = row
	tokens.mu.Unlock()

	valid, err := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: resp.GetAccessToken()})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if valid.GetValid() {
		t.Error("expired access token must not validate")
	}
}

// Single-use refresh rotation: a valid refresh token yields a new pair; the
// presented refresh token can never be reused.
func TestRefreshRotation(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("Admin A", "admin-a@example.com", "password-a", UserRoleAdmin); err != nil {
		t.Fatalf("seed: %v", err)
	}
	signInResp, err := signIn(t, svc, "admin-a@example.com", "password-a")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}

	refreshResp, err := svc.RefreshToken(context.Background(), &clientsgrpc.RefreshTokenRequest{RefreshToken: signInResp.GetRefreshToken()})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshResp.GetAccessToken() == "" || refreshResp.GetRefreshToken() == "" {
		t.Fatal("refresh must issue a new token pair")
	}
	if refreshResp.GetAccessToken() == signInResp.GetAccessToken() {
		t.Error("rotation must issue a NEW access token")
	}
	if refreshResp.GetUser().GetUserRole() != string(UserRoleAdmin) {
		t.Errorf("refreshed identity role = %q, want admin", refreshResp.GetUser().GetUserRole())
	}

	// Old refresh token is single-use: replay must fail.
	if _, err := svc.RefreshToken(context.Background(), &clientsgrpc.RefreshTokenRequest{RefreshToken: signInResp.GetRefreshToken()}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("refresh replay code = %v, want Unauthenticated", status.Code(err))
	}

	// The rotated access token validates.
	valid, err := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: refreshResp.GetAccessToken()})
	if err != nil || !valid.GetValid() {
		t.Fatalf("rotated token must validate (valid=%v err=%v)", valid.GetValid(), err)
	}
}

func TestLogoutInvalidates(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	if _, err := users.seed("Admin A", "admin-a@example.com", "password-a", UserRoleAdmin); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resp, err := signIn(t, svc, "admin-a@example.com", "password-a")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}

	authCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"grpcgateway-authorization", "Bearer "+resp.GetAccessToken(),
	))
	if _, err := svc.SignOut(authCtx, &clientsgrpc.SignOutRequest{RefreshToken: resp.GetRefreshToken()}); err != nil {
		t.Fatalf("sign-out: %v", err)
	}

	valid, err := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: resp.GetAccessToken()})
	if err != nil {
		t.Fatalf("validate after sign-out: %v", err)
	}
	if valid.GetValid() {
		t.Error("access token must be invalid after sign-out")
	}
	// The whole session dies: the refresh token is unusable too.
	if _, err := svc.RefreshToken(context.Background(), &clientsgrpc.RefreshTokenRequest{RefreshToken: resp.GetRefreshToken()}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("refresh after sign-out code = %v, want Unauthenticated", status.Code(err))
	}
	// Sign-out is idempotent.
	if _, err := svc.SignOut(authCtx, &clientsgrpc.SignOutRequest{}); err != nil {
		t.Fatalf("repeated sign-out must be idempotent: %v", err)
	}
}

// No bootstrap: startup must NOT create an administrator merely because the
// users table is empty. Constructing the service performs no writes and no
// sign-in is possible against an empty table.
func TestNoBootstrapOnEmptyUsersTable(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	_ = newTestService(users, tokens) // construction alone must not provision

	if users.count() != 0 {
		t.Fatalf("service construction created users; users table must remain empty")
	}
	if _, err := signIn(t, newTestService(users, tokens), "anything@example.com", "whatever"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sign-in against empty users table must fail with Unauthenticated, got %v", err)
	}
	if users.count() != 0 {
		t.Fatalf("failed sign-in must not create users")
	}
}

// The operator hash tool's contract: the generated hash verifies against the
// same password and is a well-formed PHC argon2id string.
func TestHashPasswordOperatorContract(t *testing.T) {
	hash, err := HashPassword("operator-chosen-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, ArgonEncodedPrefix) {
		t.Fatalf("hash must be PHC argon2id format, got %q", hash)
	}
	if !VerifyPassword("operator-chosen-password", hash) {
		t.Error("generated hash must verify against the same password")
	}
	if VerifyPassword("wrong", hash) {
		t.Error("generated hash must not verify against a wrong password")
	}
}
