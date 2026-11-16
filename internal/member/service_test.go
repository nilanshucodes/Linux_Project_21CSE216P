package member_test

import (
	"context"
	"errors"
	"testing"

	"student-club-system/internal/member"
	"student-club-system/internal/storage"
)

func newTestService(t *testing.T) (*member.Service, *storage.SQLiteStore) {
	t.Helper()
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return member.NewService(store), store
}

func TestRegisterAndDuplicateStudentID(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	created, err := service.Register(ctx, member.NewMember{
		StudentID: "STU3001", Name: "Sample Student", Department: "Computer Science", Year: 3,
	})
	if err != nil {
		t.Fatalf("register member: %v", err)
	}
	if created.Role != member.RoleMember || created.Status != member.StatusActive {
		t.Fatalf("unexpected defaults: role=%q status=%q", created.Role, created.Status)
	}

	_, err = service.Register(ctx, member.NewMember{
		StudentID: "stu3001", Name: "Duplicate Student", Department: "IT", Year: 2,
	})
	if !errors.Is(err, member.ErrDuplicateStudent) {
		t.Fatalf("expected duplicate ID error, got %v", err)
	}
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	service, _ := newTestService(t)
	_, err := service.Register(context.Background(), member.NewMember{
		StudentID: "STU3002", Name: "", Department: "Computer Science", Year: 2,
	})
	if !errors.Is(err, member.ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestListSearchAssignRoleAndDeactivate(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	_, err := service.Register(ctx, member.NewMember{
		StudentID: "STU3003", Name: "Aarav Mehta", Department: "Computer Science", Year: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Register(ctx, member.NewMember{
		StudentID: "STU3004", Name: "Diya Nair", Department: "Electronics", Year: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	all, err := service.List(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("List() got %d members, err=%v; want 2", len(all), err)
	}
	found, err := service.Search(ctx, "aarav")
	if err != nil || len(found) != 1 || found[0].StudentID != "STU3003" {
		t.Fatalf("Search() got %#v, err=%v", found, err)
	}

	if err := service.AssignRole(ctx, "STU3003", member.RolePresident); err != nil {
		t.Fatalf("assign role: %v", err)
	}
	if err := service.Deactivate(ctx, "STU3003"); err != nil {
		t.Fatalf("deactivate member: %v", err)
	}
	// Repeating deactivation is safe and must not delete the record.
	if err := service.Deactivate(ctx, "STU3003"); err != nil {
		t.Fatalf("repeat deactivation: %v", err)
	}

	got, err := service.GetByStudentID(ctx, "STU3003")
	if err != nil {
		t.Fatalf("get retained member: %v", err)
	}
	if got.Role != member.RolePresident || got.Status != member.StatusInactive {
		t.Fatalf("unexpected updated member: role=%q status=%q", got.Role, got.Status)
	}
}

func TestAssignRoleRejectsInvalidRole(t *testing.T) {
	service, _ := newTestService(t)
	err := service.AssignRole(context.Background(), "STU9999", member.Role("Administrator"))
	if !errors.Is(err, member.ErrInvalidRole) {
		t.Fatalf("expected invalid role error, got %v", err)
	}
}

func TestCountsByRoleAndStatus(t *testing.T) {
	service, store := newTestService(t)
	ctx := context.Background()
	seeded, err := store.SeedIfEmpty(ctx)
	if err != nil || !seeded {
		t.Fatalf("SeedIfEmpty() seeded=%v err=%v; want true and nil", seeded, err)
	}
	// Seeding a populated database must not add duplicate sample records.
	seeded, err = store.SeedIfEmpty(ctx)
	if err != nil || seeded {
		t.Fatalf("second SeedIfEmpty() seeded=%v err=%v; want false and nil", seeded, err)
	}

	counts, err := service.Counts(ctx)
	if err != nil {
		t.Fatalf("get counts: %v", err)
	}
	if counts.Total != 8 {
		t.Fatalf("total=%d; want 8", counts.Total)
	}
	roleTotal, statusTotal := 0, 0
	for _, item := range counts.ByRole {
		roleTotal += item.Count
	}
	for _, item := range counts.ByStatus {
		statusTotal += item.Count
	}
	if roleTotal != counts.Total || statusTotal != counts.Total {
		t.Fatalf("group counts do not add up: total=%d role=%d status=%d", counts.Total, roleTotal, statusTotal)
	}
}
