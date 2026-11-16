package member

import (
	"context"
	"fmt"
	"strings"
)

// Repository defines the persistence operations needed by the membership service.
type Repository interface {
	Create(context.Context, NewMember, Role) (Member, error)
	List(context.Context) ([]Member, error)
	Search(context.Context, string) ([]Member, error)
	GetByStudentID(context.Context, string) (Member, error)
	UpdateRole(context.Context, string, Role) error
	Deactivate(context.Context, string) error
	GetCounts(context.Context) (Counts, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, input NewMember) (Member, error) {
	input.StudentID = strings.TrimSpace(input.StudentID)
	input.Name = strings.TrimSpace(input.Name)
	input.Department = strings.TrimSpace(input.Department)

	if input.StudentID == "" || input.Name == "" || input.Department == "" {
		return Member{}, fmt.Errorf("%w: student ID, name, and department are required", ErrInvalidInput)
	}
	if input.Year < 1 || input.Year > 6 {
		return Member{}, fmt.Errorf("%w: year must be between 1 and 6", ErrInvalidInput)
	}

	return s.repo.Create(ctx, input, RoleMember)
}

func (s *Service) List(ctx context.Context) ([]Member, error) {
	return s.repo.List(ctx)
}

func (s *Service) Search(ctx context.Context, query string) ([]Member, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("%w: enter a student ID or name to search", ErrInvalidInput)
	}
	return s.repo.Search(ctx, query)
}

func (s *Service) GetByStudentID(ctx context.Context, studentID string) (Member, error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return Member{}, fmt.Errorf("%w: student ID is required", ErrInvalidInput)
	}
	return s.repo.GetByStudentID(ctx, studentID)
}

func (s *Service) AssignRole(ctx context.Context, studentID string, role Role) error {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return fmt.Errorf("%w: student ID is required", ErrInvalidInput)
	}
	if !isAllowedRole(role) {
		return fmt.Errorf("%w: %q is not one of the supported roles", ErrInvalidRole, role)
	}
	return s.repo.UpdateRole(ctx, studentID, role)
}

func (s *Service) Deactivate(ctx context.Context, studentID string) error {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return fmt.Errorf("%w: student ID is required", ErrInvalidInput)
	}
	return s.repo.Deactivate(ctx, studentID)
}

func (s *Service) Counts(ctx context.Context) (Counts, error) {
	return s.repo.GetCounts(ctx)
}

func isAllowedRole(role Role) bool {
	for _, allowed := range AllowedRoles {
		if role == allowed {
			return true
		}
	}
	return false
}
