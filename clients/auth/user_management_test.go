package auth

import (
	"context"
	"os"
	"strings"
	"testing"

	clientsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mkUser(t *testing.T, svc *Service, name, email, password, role string) (*clientsgrpc.CreateUserResponse, error) {
	t.Helper()
	return svc.CreateUser(context.Background(), &clientsgrpc.CreateUserRequest{
		Name:     name,
		Email:    email,
		Password: password,
		UserRole: role,
	})
}

func TestCRUDCreateUserRoles(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	admin, err := mkUser(t, svc, "RVPay Admin", "admin@example.com", "admin-password-1", string(UserRoleAdmin))
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if got := admin.GetUser().GetUserRole(); got != string(UserRoleAdmin) {
		t.Errorf("admin role = %q, want %q", got, string(UserRoleAdmin))
	}
	u, err := mkUser(t, svc, "Regular User", "user@example.com", "user-password-1", string(UserRoleUser))
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if got := u.GetUser().GetUserRole(); got != string(UserRoleUser) {
		t.Errorf("user role = %q,v want %q", got, string(UserRoleUser))
	}
}

func TestCRUDHashesAreSalted(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	if _, err := mkUser(t, svc, "Admin A", "admin-a@example.com", "same-password-1", string(UserRoleAdmin)); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := mkUser(t, svc, "Admin B", "admin-b@example.com", "same-password-1", string(UserRoleAdmin)); err != nil {
		t.Fatalf("create b: %v", err)
	}
	pa := users.users["admin-a@example.com"].PasswordHash
	pb := users.users["admin-b@example.com"].PasswordHash
	if !strings.HasPrefix(pa, "$argon2id$") {
		t.Errorf("stored password must be Argon2id, got %q", pa)
	}
	if pa == pb {
		t.Error("same plaintext must produce different hashes per user (unique salt)")
	}
}

func TestCRUDDuplicateEmail(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	if _, err := mkUser(t, svc, "Admin A", "admin@example.com", "admin-password-1", string(UserRoleAdmin)); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, err := mkUser(t, svc, "Admin B", "admin@example.com", "admin-password-2", string(UserRoleAdmin))
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("duplicate email code = %v,v want AlreadyExists", status.Code(err))
	}
}

func TestCRUDResponseHidesSecrets(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	resp, _ := mkUser(t, svc, "Admin", "admin@example.com", "admin-password-1", string(UserRoleAdmin))
	body := resp.String()
	if strings.Contains(body, "admin-password-1") || strings.Contains(body, "$argon2id$") {
		t.Error("CreateUserResponse must not expose password or hash material")
	}
}
func TestCRUDUpdateUserNameEmail(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	created, err := mkUser(t, svc, "Old Name", "old@example.com", "admin-password-1", string(UserRoleAdmin))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	resp, err := svc.UpdateUser(context.Background(), &clientsgrpc.UpdateUserRequest{
		Id:    created.GetUser().GetId(),
		Name:  "New Name",
		Email: "new@example.com",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.GetUser().GetName() != "New Name" || resp.GetUser().GetEmail() != "new@example.com" {
		t.Errorf("unexpected identity after update: %+v", resp.GetUser())
	}
	if resp.GetUser().GetUserRole() != string(UserRoleAdmin) {
		t.Errorf("role must be immutable through UpdateUser; got %q", resp.GetUser().GetUserRole())
	}
	if _, err := signIn(t, svc, "new@example.com", "admin-password-1"); err != nil {
		t.Errorf("sign-in after rename: %v", err)
	}
	if _, err := signIn(t, svc, "old@example.com", "admin-password-1"); status.Code(err) != codes.Unauthenticated {
		t.Error("old email must no longer authenticate")
	}
}

func TestCRUDUpdateUserPasswordInvalidatesSessions(t *testing.T) {
	users := newFakeUserRepo()
	tokens := newFakeAccessTokenRepo()
	svc := newTestService(users, tokens)
	created, err := mkUser(t, svc, "Admin", "admin@example.com", "old-password-1", string(UserRoleAdmin))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	before, err := signIn(t, svc, "admin@example.com", "old-password-1")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if vr, verr := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: before.GetAccessToken()}); verr != nil || !vr.GetValid() {
		t.Fatalf("pre-change session should validate; valid=%v err=%v", vr.GetValid(), verr)
	}
	upd, err := svc.UpdateUser(context.Background(), &clientsgrpc.UpdateUserRequest{Id: created.GetUser().GetId(), Password: "new-password-99"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if strings.Contains(upd.String(), "$argon2id$") || strings.Contains(upd.String(), "new-password-99") {
		t.Error("UpdateUserResponse must never carry password or hash material")
	}
	if vr, verr := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: before.GetAccessToken()}); verr != nil {
		t.Fatalf("validate call failed: %v", verr)
	} else if vr.GetValid() {
		t.Error("pre-change access token must be invalidated after password change")
	}
	if _, rerr := svc.RefreshToken(context.Background(), &clientsgrpc.RefreshTokenRequest{RefreshToken: before.GetRefreshToken()}); status.Code(rerr) != codes.Unauthenticated {
		t.Error("old refresh token must be rejected after password change")
	}
	if _, serr := signIn(t, svc, "admin@example.com", "old-password-1"); status.Code(serr) != codes.Unauthenticated {
		t.Error("old password must no longer authenticate")
	}
	after, err := signIn(t, svc, "admin@example.com", "new-password-99")
	if err != nil {
		t.Fatalf("sign-in with new password: %v", err)
	}
	if vr, verr := svc.ValidateAccessToken(context.Background(), &clientsgrpc.ValidateAccessTokenRequest{AccessToken: after.GetAccessToken()}); verr != nil || !vr.GetValid() {
		t.Errorf("new session must validate; valid=%v err=%v", vr.GetValid(), verr)
	}
}
func TestCRUDUpdateUserDuplicateEmail(t *testing.T) {
	users := newFakeUserRepo()
	svc := newTestService(users, newFakeAccessTokenRepo())
	a, err := mkUser(t, svc, "Admin A", "admin-a@example.com", "admin-password-1", string(UserRoleAdmin))
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := mkUser(t, svc, "Admin B", "admin-b@example.com", "admin-password-2", string(UserRoleAdmin)); err != nil {
		t.Fatalf("create b: %v", err)
	}
	if _, err := svc.UpdateUser(context.Background(), &clientsgrpc.UpdateUserRequest{Id: a.GetUser().GetId(), Email: "admin-b@example.com"}); status.Code(err) != codes.AlreadyExists {
		t.Error("changing email to an existing user's email must be rejected")
	}
}

func TestCRUDMethodsAreGRPCOnly(t *testing.T) {
	protoBytes, err := os.ReadFile("../../protobuf/clients.proto")
	if err != nil {
		t.Fatalf("read proto: %v", err)
	}
	proto := string(protoBytes)
	for _, rpc := range []string{"rpc CreateUser(CreateUserRequest)", "rpc UpdateUser(UpdateUserRequest)"} {
		if !strings.Contains(proto, rpc) {
			t.Errorf("%s missing from clients.proto AuthService", rpc)
		}
	}
	gwBytes, err := os.ReadFile("../../grpc/go/clientsgrpc/clients.pb.gw.go")
	if err != nil {
		t.Fatalf("read gateway: %v", err)
	}
	gw := string(gwBytes)
	if strings.Contains(gw, "CreateUser") || strings.Contains(gw, "UpdateUser") {
		t.Error("CreateUser/UpdateUser must not appear in grpc-gateway routes")
	}
}
