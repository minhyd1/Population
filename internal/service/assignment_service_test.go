package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"population-service/internal/model"
	jwtpkg "population-service/pkg/jwt"
)

// ─── Mocks ────────────────────────────────────────────────────────────────
//
// Đây là ví dụ minh họa cho luận điểm "service phụ thuộc interface nên mock
// test dễ": AdminUnitRepository/UserRepository đã là interface từ trước, nên
// không cần thư viện mock nào — chỉ cần struct implement đúng interface.

type mockAdminUnitRepo struct {
	units       map[string]*model.AdministrativeUnit
	assignments []model.UserAssignment

	createAssignmentErr error
	createdAssignment   *model.UserAssignment
}

func (m *mockAdminUnitRepo) FindUnitByCode(ctx context.Context, code string) (*model.AdministrativeUnit, error) {
	u, ok := m.units[code]
	if !ok {
		return nil, nil
	}
	return u, nil
}
func (m *mockAdminUnitRepo) GetParentUnit(ctx context.Context, code string) (*model.AdministrativeUnit, error) {
	return nil, nil
}
func (m *mockAdminUnitRepo) GetChildUnits(ctx context.Context, parentCode string) ([]model.AdministrativeUnit, error) {
	return nil, nil
}
func (m *mockAdminUnitRepo) GetAncestorCodes(ctx context.Context, code string) ([]string, error) {
	return nil, nil
}
func (m *mockAdminUnitRepo) GetDescendantCodes(ctx context.Context, code string) ([]string, error) {
	return nil, nil
}
func (m *mockAdminUnitRepo) CreateAssignment(ctx context.Context, a *model.UserAssignment) error {
	if m.createAssignmentErr != nil {
		return m.createAssignmentErr
	}
	m.createdAssignment = a
	return nil
}
func (m *mockAdminUnitRepo) EndAssignment(ctx context.Context, id string, endDate time.Time, note *string) error {
	return nil
}
func (m *mockAdminUnitRepo) GetActiveAssignments(ctx context.Context, userID string) ([]model.UserAssignment, error) {
	return m.assignments, nil
}
func (m *mockAdminUnitRepo) GetAssignmentHistory(ctx context.Context, userID string) ([]model.UserAssignment, error) {
	return m.assignments, nil
}
func (m *mockAdminUnitRepo) GetActiveOfficersByUnit(ctx context.Context, unitCode string) ([]model.UserAssignment, error) {
	return m.assignments, nil
}
func (m *mockAdminUnitRepo) GetOfficerAtTime(ctx context.Context, unitCode string, at time.Time) ([]model.UserAssignment, error) {
	return m.assignments, nil
}
func (m *mockAdminUnitRepo) GetRolePermissions(ctx context.Context, role string) ([]string, error) {
	return nil, nil
}
func (m *mockAdminUnitRepo) HasPermission(ctx context.Context, role, permissionCode string) (bool, error) {
	return false, nil
}

type mockUserRepo struct {
	users map[string]*model.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error { return nil }
func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}
func (m *mockUserRepo) ListUsers(ctx context.Context) ([]*model.User, error) { return nil, nil }
func (m *mockUserRepo) UpdateRole(ctx context.Context, userID string, role jwtpkg.Role, provinceCode, districtCode, wardCode *string) error {
	return nil
}
func (m *mockUserRepo) SetActive(ctx context.Context, userID string, active bool) error { return nil }
func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID, newHash string) error {
	return nil
}
func (m *mockUserRepo) SaveRefreshToken(ctx context.Context, rt *model.RefreshToken) error {
	return nil
}
func (m *mockUserRepo) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	return nil, nil
}
func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error { return nil }
func (m *mockUserRepo) RevokeAllUserTokens(ctx context.Context, userID string) error   { return nil }

// ─── Tests ────────────────────────────────────────────────────────────────

func TestAssignUser_Success(t *testing.T) {
	adminRepo := &mockAdminUnitRepo{
		units: map[string]*model.AdministrativeUnit{
			"HN-BD": {Code: "HN-BD", Name: "Phường Ba Đình"},
		},
	}
	userRepo := &mockUserRepo{
		users: map[string]*model.User{
			"user-1": {ID: "user-1", Username: "an.nguyen", Role: jwtpkg.RoleWardOfficer},
		},
	}
	svc := NewAssignmentService(adminRepo, userRepo)

	req := model.AssignUserRequest{
		UserID:    "user-1",
		UnitCode:  "HN-BD",
		StartDate: "2026-01-01",
	}

	got, err := svc.AssignUser(context.Background(), req, "admin-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.UnitCode != "HN-BD" {
		t.Errorf("expected UnitCode=HN-BD, got %s", got.UnitCode)
	}
	if got.Role != string(jwtpkg.RoleWardOfficer) {
		t.Errorf("expected Role snapshot=%s, got %s", jwtpkg.RoleWardOfficer, got.Role)
	}
	if adminRepo.createdAssignment == nil {
		t.Fatal("expected CreateAssignment to be called")
	}
}

func TestAssignUser_UnitNotFound(t *testing.T) {
	adminRepo := &mockAdminUnitRepo{units: map[string]*model.AdministrativeUnit{}}
	userRepo := &mockUserRepo{users: map[string]*model.User{
		"user-1": {ID: "user-1"},
	}}
	svc := NewAssignmentService(adminRepo, userRepo)

	_, err := svc.AssignUser(context.Background(), model.AssignUserRequest{
		UserID:    "user-1",
		UnitCode:  "KHONG-TON-TAI",
		StartDate: "2026-01-01",
	}, "admin-1")

	if err == nil {
		t.Fatal("expected error for nonexistent unit_code, got nil")
	}
}

func TestAssignUser_UserNotFound(t *testing.T) {
	adminRepo := &mockAdminUnitRepo{units: map[string]*model.AdministrativeUnit{
		"HN-BD": {Code: "HN-BD"},
	}}
	userRepo := &mockUserRepo{users: map[string]*model.User{}}
	svc := NewAssignmentService(adminRepo, userRepo)

	_, err := svc.AssignUser(context.Background(), model.AssignUserRequest{
		UserID:    "khong-ton-tai",
		UnitCode:  "HN-BD",
		StartDate: "2026-01-01",
	}, "admin-1")

	if err == nil {
		t.Fatal("expected error for nonexistent user, got nil")
	}
}

func TestAssignUser_InvalidStartDate(t *testing.T) {
	adminRepo := &mockAdminUnitRepo{units: map[string]*model.AdministrativeUnit{
		"HN-BD": {Code: "HN-BD"},
	}}
	userRepo := &mockUserRepo{users: map[string]*model.User{
		"user-1": {ID: "user-1"},
	}}
	svc := NewAssignmentService(adminRepo, userRepo)

	_, err := svc.AssignUser(context.Background(), model.AssignUserRequest{
		UserID:    "user-1",
		UnitCode:  "HN-BD",
		StartDate: "01-01-2026", // sai định dạng, đúng phải là 2006-01-02
	}, "admin-1")

	if err == nil {
		t.Fatal("expected error for invalid start_date format, got nil")
	}
}

func TestAssignUser_RepositoryError_Propagates(t *testing.T) {
	adminRepo := &mockAdminUnitRepo{
		units: map[string]*model.AdministrativeUnit{
			"HN-BD": {Code: "HN-BD"},
		},
		createAssignmentErr: errors.New("db lỗi kết nối"),
	}
	userRepo := &mockUserRepo{users: map[string]*model.User{
		"user-1": {ID: "user-1"},
	}}
	svc := NewAssignmentService(adminRepo, userRepo)

	_, err := svc.AssignUser(context.Background(), model.AssignUserRequest{
		UserID:    "user-1",
		UnitCode:  "HN-BD",
		StartDate: "2026-01-01",
	}, "admin-1")

	if err == nil {
		t.Fatal("expected repository error to propagate, got nil")
	}
}

func TestGetActiveUnitCodes(t *testing.T) {
	now := time.Now()
	adminRepo := &mockAdminUnitRepo{
		assignments: []model.UserAssignment{
			{ID: "a1", UserID: "user-1", UnitCode: "HN-BD", StartDate: now}, // EndDate nil -> active
			{ID: "a2", UserID: "user-1", UnitCode: "HN-HK", StartDate: now, EndDate: &now}, // đã kết thúc
		},
	}
	userRepo := &mockUserRepo{users: map[string]*model.User{}}
	svc := NewAssignmentService(adminRepo, userRepo)

	codes, err := svc.GetActiveUnitCodes(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	found := false
	for _, c := range codes {
		if c == "HN-BD" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected active unit HN-BD in result, got %v", codes)
	}
}
