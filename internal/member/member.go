package member

import "time"

// Role is a predefined responsibility a student can hold in the club.
type Role string

const (
	RolePresident        Role = "President"
	RoleVicePresident    Role = "Vice President"
	RoleSecretary        Role = "Secretary"
	RoleTreasurer        Role = "Treasurer"
	RoleEventCoordinator Role = "Event Coordinator"
	RoleMember           Role = "Member"
)

var AllowedRoles = []Role{
	RolePresident,
	RoleVicePresident,
	RoleSecretary,
	RoleTreasurer,
	RoleEventCoordinator,
	RoleMember,
}

const (
	StatusActive   = "Active"
	StatusInactive = "Inactive"
)

// Member represents one student's membership record in a club.
type Member struct {
	ID         int64     `json:"id"`
	StudentID  string    `json:"student_id"`
	Name       string    `json:"name"`
	Department string    `json:"department"`
	Year       int       `json:"year"`
	Role       Role      `json:"role"`
	Status     string    `json:"status"`
	JoinedAt   time.Time `json:"joined_at"`
}

type NewMember struct {
	StudentID  string
	Name       string
	Department string
	Year       int
}

type RoleCount struct {
	Role  Role
	Count int
}

type StatusCount struct {
	Status string
	Count  int
}

type Counts struct {
	Total    int
	ByRole   []RoleCount
	ByStatus []StatusCount
}
