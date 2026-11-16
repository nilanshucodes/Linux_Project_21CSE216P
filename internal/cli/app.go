package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"student-club-system/internal/member"
)

type MembershipService interface {
	Register(context.Context, member.NewMember) (member.Member, error)
	List(context.Context) ([]member.Member, error)
	Search(context.Context, string) ([]member.Member, error)
	GetByStudentID(context.Context, string) (member.Member, error)
	AssignRole(context.Context, string, member.Role) error
	Deactivate(context.Context, string) error
	Counts(context.Context) (member.Counts, error)
}

type App struct {
	service MembershipService
	in      *bufio.Reader
	out     io.Writer
}

func NewApp(service MembershipService, in io.Reader, out io.Writer) *App {
	return &App{service: service, in: bufio.NewReader(in), out: out}
}

func (a *App) Run(ctx context.Context) error {
	fmt.Fprintln(a.out, "\n========================================")
	fmt.Fprintln(a.out, "     STUDENT CLUB MEMBERSHIP SYSTEM")
	fmt.Fprintln(a.out, "========================================")

	for {
		a.printMenu()
		choice, err := a.readLine("Select an option: ")
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(a.out, "\nInput closed. Exiting.")
			return nil
		}
		if err != nil {
			return err
		}

		var actionErr error
		switch choice {
		case "1":
			actionErr = a.register(ctx)
		case "2":
			actionErr = a.list(ctx)
		case "3":
			actionErr = a.search(ctx)
		case "4":
			actionErr = a.assignRole(ctx)
		case "5":
			actionErr = a.deactivate(ctx)
		case "6":
			actionErr = a.statistics(ctx)
		case "7":
			fmt.Fprintln(a.out, "Exiting Student Club Membership System.")
			return nil
		default:
			fmt.Fprintln(a.out, "Invalid option. Select a number from 1 to 7.")
		}
		if actionErr != nil {
			fmt.Fprintf(a.out, "Error: %v\n", actionErr)
		}
	}
}

func (a *App) printMenu() {
	fmt.Fprintln(a.out, "\n--------------- MAIN MENU ---------------")
	fmt.Fprintln(a.out, "1. Register a new member")
	fmt.Fprintln(a.out, "2. List all members")
	fmt.Fprintln(a.out, "3. Search members")
	fmt.Fprintln(a.out, "4. Assign / update a club role")
	fmt.Fprintln(a.out, "5. Deactivate membership")
	fmt.Fprintln(a.out, "6. View membership statistics")
	fmt.Fprintln(a.out, "7. Exit")
}

func (a *App) register(ctx context.Context) error {
	fmt.Fprintln(a.out, "\n--- Register Member ---")
	studentID, err := a.readLine("Student ID: ")
	if err != nil {
		return err
	}
	name, err := a.readLine("Full name: ")
	if err != nil {
		return err
	}
	department, err := a.readLine("Department: ")
	if err != nil {
		return err
	}
	yearText, err := a.readLine("Academic year (1-6): ")
	if err != nil {
		return err
	}
	year, err := strconv.Atoi(yearText)
	if err != nil {
		return fmt.Errorf("academic year must be a number between 1 and 6")
	}
	created, err := a.service.Register(ctx, member.NewMember{
		StudentID: studentID, Name: name, Department: department, Year: year,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "Member registered successfully. Assigned role: %s | Status: %s\n", created.Role, created.Status)
	return nil
}

func (a *App) list(ctx context.Context) error {
	members, err := a.service.List(ctx)
	if err != nil {
		return err
	}
	printMembers(a.out, members)
	return nil
}

func (a *App) search(ctx context.Context) error {
	query, err := a.readLine("Enter student ID, name, or department: ")
	if err != nil {
		return err
	}
	members, err := a.service.Search(ctx, query)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		fmt.Fprintln(a.out, "No matching members found.")
		return nil
	}
	printMembers(a.out, members)
	return nil
}

func (a *App) assignRole(ctx context.Context) error {
	studentID, err := a.readLine("Student ID: ")
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "Available roles:")
	for i, role := range member.AllowedRoles {
		fmt.Fprintf(a.out, "%d. %s\n", i+1, role)
	}
	roleText, err := a.readLine("Choose role number: ")
	if err != nil {
		return err
	}
	index, err := strconv.Atoi(roleText)
	if err != nil || index < 1 || index > len(member.AllowedRoles) {
		return errors.New("invalid role selection")
	}
	role := member.AllowedRoles[index-1]
	if err := a.service.AssignRole(ctx, studentID, role); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "Role for %s updated to %s.\n", strings.TrimSpace(studentID), role)
	return nil
}

func (a *App) deactivate(ctx context.Context) error {
	studentID, err := a.readLine("Student ID to deactivate: ")
	if err != nil {
		return err
	}
	if err := a.service.Deactivate(ctx, studentID); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "Membership for %s is now inactive. The member record was retained.\n", strings.TrimSpace(studentID))
	return nil
}

func (a *App) statistics(ctx context.Context) error {
	counts, err := a.service.Counts(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "\n--- Membership Statistics ---")
	fmt.Fprintf(a.out, "Total members: %d\n", counts.Total)
	fmt.Fprintln(a.out, "\nCounts by role:")
	if len(counts.ByRole) == 0 {
		fmt.Fprintln(a.out, "  No member records.")
	}
	for _, item := range counts.ByRole {
		fmt.Fprintf(a.out, "  %-20s %d\n", item.Role, item.Count)
	}
	fmt.Fprintln(a.out, "\nCounts by membership status:")
	if len(counts.ByStatus) == 0 {
		fmt.Fprintln(a.out, "  No member records.")
	}
	for _, item := range counts.ByStatus {
		fmt.Fprintf(a.out, "  %-20s %d\n", item.Status, item.Count)
	}
	return nil
}

func (a *App) readLine(prompt string) (string, error) {
	fmt.Fprint(a.out, prompt)
	line, err := a.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}
	// A final unterminated line is still valid input; the next read will return EOF.
	return line, nil
}

func printMembers(out io.Writer, members []member.Member) {
	if len(members) == 0 {
		fmt.Fprintln(out, "No members registered.")
		return
	}
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "STUDENT ID\tNAME\tDEPARTMENT\tYEAR\tROLE\tSTATUS\tJOINED")
	for _, item := range members {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			item.StudentID, item.Name, item.Department, item.Year,
			item.Role, item.Status, item.JoinedAt.Format(time.DateOnly))
	}
	_ = writer.Flush()
}
