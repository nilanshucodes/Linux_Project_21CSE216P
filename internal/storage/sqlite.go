package storage

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int club_bind_text(sqlite3_stmt *stmt, int index, const char *value) {
    return sqlite3_bind_text(stmt, index, value, -1, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"student-club-system/internal/member"
)

// SQLiteStore implements membership persistence through the system SQLite C library.
// A single mutex serializes access because this CLI has one interactive user.
type SQLiteStore struct {
	mu sync.Mutex
	db *C.sqlite3
}

func Open(path string) (*SQLiteStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path cannot be empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var db *C.sqlite3
	if rc := C.sqlite3_open(cPath, &db); rc != C.SQLITE_OK {
		message := "unknown SQLite error"
		if db != nil {
			message = C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close(db)
		}
		return nil, fmt.Errorf("open SQLite database: %s", message)
	}
	store := &SQLiteStore{db: db}
	C.sqlite3_busy_timeout(db, 5000)
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	} {
		if err := store.execLocked(pragma); err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("configure SQLite (%s): %w", pragma, err)
		}
	}
	if err := store.initSchema(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(s.db); rc != C.SQLITE_OK {
		return fmt.Errorf("close SQLite database: %s", C.GoString(C.sqlite3_errmsg(s.db)))
	}
	s.db = nil
	return nil
}

func (s *SQLiteStore) initSchema() error {
	const schema = `
CREATE TABLE IF NOT EXISTS members (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id  TEXT NOT NULL COLLATE NOCASE UNIQUE,
    name        TEXT NOT NULL,
    department  TEXT NOT NULL,
    year        INTEGER NOT NULL CHECK (year BETWEEN 1 AND 6),
    role        TEXT NOT NULL CHECK (role IN ('President', 'Vice President', 'Secretary', 'Treasurer', 'Event Coordinator', 'Member')),
    status      TEXT NOT NULL DEFAULT 'Active' CHECK (status IN ('Active', 'Inactive')),
    joined_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_members_name ON members(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_members_role ON members(role);
CREATE INDEX IF NOT EXISTS idx_members_status ON members(status);
`
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked(schema); err != nil {
		return fmt.Errorf("initialize database schema: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Create(ctx context.Context, input member.NewMember, role member.Role) (member.Member, error) {
	if err := checkContext(ctx); err != nil {
		return member.Member{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	joinedAt := time.Now().UTC()
	stmt, err := s.prepareLocked(`INSERT INTO members
(student_id, name, department, year, role, status, joined_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return member.Member{}, fmt.Errorf("prepare member insert: %w", err)
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindText(stmt, 1, input.StudentID); err != nil {
		return member.Member{}, err
	}
	if err := bindText(stmt, 2, input.Name); err != nil {
		return member.Member{}, err
	}
	if err := bindText(stmt, 3, input.Department); err != nil {
		return member.Member{}, err
	}
	C.sqlite3_bind_int(stmt, 4, C.int(input.Year))
	if err := bindText(stmt, 5, string(role)); err != nil {
		return member.Member{}, err
	}
	if err := bindText(stmt, 6, member.StatusActive); err != nil {
		return member.Member{}, err
	}
	if err := bindText(stmt, 7, joinedAt.Format(time.RFC3339Nano)); err != nil {
		return member.Member{}, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_DONE {
		message := s.errorMessageLocked()
		if strings.Contains(strings.ToLower(message), "unique constraint failed") {
			return member.Member{}, member.ErrDuplicateStudent
		}
		return member.Member{}, fmt.Errorf("insert member: %s", message)
	}
	return member.Member{
		ID: int64(C.sqlite3_last_insert_rowid(s.db)), StudentID: input.StudentID,
		Name: input.Name, Department: input.Department, Year: input.Year,
		Role: role, Status: member.StatusActive, JoinedAt: joinedAt,
	}, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]member.Member, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queryMembersLocked(`SELECT id, student_id, name, department, year, role, status, joined_at
FROM members ORDER BY CASE status WHEN 'Active' THEN 0 ELSE 1 END, name COLLATE NOCASE`)
}

func (s *SQLiteStore) Search(ctx context.Context, query string) ([]member.Member, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pattern := "%" + query + "%"
	return s.queryMembersLocked(`SELECT id, student_id, name, department, year, role, status, joined_at
FROM members WHERE student_id LIKE ? OR name LIKE ? OR department LIKE ?
ORDER BY CASE status WHEN 'Active' THEN 0 ELSE 1 END, name COLLATE NOCASE`, pattern, pattern, pattern)
}

func (s *SQLiteStore) GetByStudentID(ctx context.Context, studentID string) (member.Member, error) {
	if err := checkContext(ctx); err != nil {
		return member.Member{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	members, err := s.queryMembersLocked(`SELECT id, student_id, name, department, year, role, status, joined_at
FROM members WHERE student_id = ?`, studentID)
	if err != nil {
		return member.Member{}, err
	}
	if len(members) == 0 {
		return member.Member{}, member.ErrNotFound
	}
	return members[0], nil
}

func (s *SQLiteStore) UpdateRole(ctx context.Context, studentID string, role member.Role) error {
	return s.updateOne(ctx, `UPDATE members SET role = ? WHERE student_id = ?`, string(role), studentID)
}

func (s *SQLiteStore) Deactivate(ctx context.Context, studentID string) error {
	// Idempotent: repeating deactivation leaves the member inactive, not deleted.
	return s.updateOne(ctx, `UPDATE members SET status = ? WHERE student_id = ?`, member.StatusInactive, studentID)
}

func (s *SQLiteStore) updateOne(ctx context.Context, query, first, second string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(query)
	if err != nil {
		return fmt.Errorf("prepare update: %w", err)
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindText(stmt, 1, first); err != nil {
		return err
	}
	if err := bindText(stmt, 2, second); err != nil {
		return err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_DONE {
		return fmt.Errorf("update member: %s", s.errorMessageLocked())
	}
	if C.sqlite3_changes(s.db) == 0 {
		return member.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) GetCounts(ctx context.Context) (member.Counts, error) {
	if err := checkContext(ctx); err != nil {
		return member.Counts{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var counts member.Counts
	stmt, err := s.prepareLocked(`SELECT COUNT(*) FROM members`)
	if err != nil {
		return counts, err
	}
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_ROW {
		counts.Total = int(C.sqlite3_column_int(stmt, 0))
	} else {
		C.sqlite3_finalize(stmt)
		return counts, fmt.Errorf("count members: %s", s.errorMessageLocked())
	}
	C.sqlite3_finalize(stmt)

	roleStmt, err := s.prepareLocked(`SELECT role, COUNT(*) FROM members GROUP BY role ORDER BY role`)
	if err != nil {
		return counts, err
	}
	for {
		rc := C.sqlite3_step(roleStmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			C.sqlite3_finalize(roleStmt)
			return counts, fmt.Errorf("read role counts: %s", s.errorMessageLocked())
		}
		counts.ByRole = append(counts.ByRole, member.RoleCount{
			Role: member.Role(columnText(roleStmt, 0)), Count: int(C.sqlite3_column_int(roleStmt, 1)),
		})
	}
	C.sqlite3_finalize(roleStmt)

	statusStmt, err := s.prepareLocked(`SELECT status, COUNT(*) FROM members GROUP BY status ORDER BY status`)
	if err != nil {
		return counts, err
	}
	for {
		rc := C.sqlite3_step(statusStmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			C.sqlite3_finalize(statusStmt)
			return counts, fmt.Errorf("read status counts: %s", s.errorMessageLocked())
		}
		counts.ByStatus = append(counts.ByStatus, member.StatusCount{
			Status: columnText(statusStmt, 0), Count: int(C.sqlite3_column_int(statusStmt, 1)),
		})
	}
	C.sqlite3_finalize(statusStmt)
	return counts, nil
}

// SeedIfEmpty inserts fictional demonstration records only into an empty database.
func (s *SQLiteStore) SeedIfEmpty(ctx context.Context) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	countStmt, err := s.prepareLocked(`SELECT COUNT(*) FROM members`)
	if err != nil {
		return false, err
	}
	if rc := C.sqlite3_step(countStmt); rc != C.SQLITE_ROW {
		C.sqlite3_finalize(countStmt)
		return false, fmt.Errorf("check seed state: %s", s.errorMessageLocked())
	}
	count := int(C.sqlite3_column_int(countStmt, 0))
	C.sqlite3_finalize(countStmt)
	if count > 0 {
		return false, nil
	}

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return false, fmt.Errorf("begin sample-data transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	type sample struct {
		id, name, department string
		year                 int
		role                 member.Role
		status               string
	}
	samples := []sample{
		{"STU1001", "Aarav Mehta", "Computer Science", 3, member.RolePresident, member.StatusActive},
		{"STU1002", "Diya Nair", "Information Technology", 2, member.RoleVicePresident, member.StatusActive},
		{"STU1003", "Kabir Rao", "Computer Science", 3, member.RoleSecretary, member.StatusActive},
		{"STU1004", "Ananya Iyer", "Electronics", 2, member.RoleTreasurer, member.StatusActive},
		{"STU1005", "Ishaan Das", "Computer Science", 4, member.RoleEventCoordinator, member.StatusActive},
		{"STU1006", "Meera Shah", "Mechanical", 1, member.RoleMember, member.StatusActive},
		{"STU1007", "Rohan Kapoor", "Information Technology", 2, member.RoleMember, member.StatusInactive},
		{"STU1008", "Tara Menon", "Computer Science", 3, member.RoleMember, member.StatusActive},
	}
	joinedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range samples {
		stmt, err := s.prepareLocked(`INSERT INTO members
(student_id, name, department, year, role, status, joined_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return false, err
		}
		bindErr := bindText(stmt, 1, item.id)
		if bindErr == nil {
			bindErr = bindText(stmt, 2, item.name)
		}
		if bindErr == nil {
			bindErr = bindText(stmt, 3, item.department)
		}
		C.sqlite3_bind_int(stmt, 4, C.int(item.year))
		if bindErr == nil {
			bindErr = bindText(stmt, 5, string(item.role))
		}
		if bindErr == nil {
			bindErr = bindText(stmt, 6, item.status)
		}
		if bindErr == nil {
			bindErr = bindText(stmt, 7, joinedAt)
		}
		if bindErr != nil {
			C.sqlite3_finalize(stmt)
			return false, bindErr
		}
		if rc := C.sqlite3_step(stmt); rc != C.SQLITE_DONE {
			message := s.errorMessageLocked()
			C.sqlite3_finalize(stmt)
			return false, fmt.Errorf("insert sample member %s: %s", item.id, message)
		}
		C.sqlite3_finalize(stmt)
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return false, fmt.Errorf("commit sample-data transaction: %w", err)
	}
	committed = true
	return true, nil
}

func (s *SQLiteStore) queryMembersLocked(query string, args ...string) ([]member.Member, error) {
	stmt, err := s.prepareLocked(query)
	if err != nil {
		return nil, fmt.Errorf("prepare member query: %w", err)
	}
	defer C.sqlite3_finalize(stmt)
	for i, arg := range args {
		if err := bindText(stmt, C.int(i+1), arg); err != nil {
			return nil, err
		}
	}
	members := make([]member.Member, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, fmt.Errorf("query members: %s", s.errorMessageLocked())
		}
		joinedAtText := columnText(stmt, 7)
		joinedAt, err := time.Parse(time.RFC3339Nano, joinedAtText)
		if err != nil {
			return nil, fmt.Errorf("parse member joined date %q: %w", joinedAtText, err)
		}
		members = append(members, member.Member{
			ID:         int64(C.sqlite3_column_int64(stmt, 0)),
			StudentID:  columnText(stmt, 1),
			Name:       columnText(stmt, 2),
			Department: columnText(stmt, 3),
			Year:       int(C.sqlite3_column_int(stmt, 4)),
			Role:       member.Role(columnText(stmt, 5)),
			Status:     columnText(stmt, 6),
			JoinedAt:   joinedAt,
		})
	}
	return members, nil
}

func (s *SQLiteStore) prepareLocked(query string) (*C.sqlite3_stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(s.db, cQuery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("%s", s.errorMessageLocked())
	}
	return stmt, nil
}

func (s *SQLiteStore) execLocked(query string) error {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var errorMessage *C.char
	rc := C.sqlite3_exec(s.db, cQuery, nil, nil, &errorMessage)
	if rc == C.SQLITE_OK {
		return nil
	}
	message := s.errorMessageLocked()
	if errorMessage != nil {
		message = C.GoString(errorMessage)
		C.sqlite3_free(unsafe.Pointer(errorMessage))
	}
	return fmt.Errorf("%s", message)
}

func (s *SQLiteStore) errorMessageLocked() string {
	if s.db == nil {
		return "database is closed"
	}
	return C.GoString(C.sqlite3_errmsg(s.db))
}

func bindText(stmt *C.sqlite3_stmt, index C.int, value string) error {
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	if rc := C.club_bind_text(stmt, index, cValue); rc != C.SQLITE_OK {
		return fmt.Errorf("bind SQLite text parameter: error code %d", int(rc))
	}
	return nil
}

func columnText(stmt *C.sqlite3_stmt, index C.int) string {
	value := C.sqlite3_column_text(stmt, index)
	if value == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(value)))
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}
	return ctx.Err()
}
