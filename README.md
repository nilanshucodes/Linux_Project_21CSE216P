# Student Club Membership System

A terminal-based membership manager built with **Go, SQLite, and Podman/Docker**. It is designed to satisfy the supplied Linux project requirements and uses fictional demonstration records.

## Requirement coverage

| Teacher's requirement | Implementation |
|---|---|
| Register a member with student ID and assigned fields | Register member with unique student ID, name, department, academic year; a new member starts with role `Member` and status `Active` |
| List and search members | List all records; search by student ID, name, or department |
| Assign a club role | Assign/update President, Vice President, Secretary, Treasurer, Event Coordinator, or Member |
| Deactivate membership | Change status to `Inactive`; the record is retained, not deleted |
| Show counts by role or membership status | Statistics screen shows total, grouped role counts, and grouped status counts |
| Container/persistence requirement | SQLite database is stored in `/data/club.db` inside the container and mounted to a host directory or named volume |
| Fictional/sample data | Eight fictional sample members are inserted only when the database is empty |

## Requirements

- Go 1.23+ and CGO enabled for running/building locally (Go 1.27 is used in the container build).
- A C compiler and SQLite development headers/library (`gcc`/`build-essential` and `libsqlite3-dev` on Debian/Ubuntu; `sqlite-devel` on Fedora).
- Podman or Docker for the container demonstration.
- Internet access when Podman/Docker first downloads the base images.

The application links directly to the system SQLite library through a small CGO layer. It does not require a separate SQLite server or third-party Go modules.

On Debian/Ubuntu, install local build prerequisites with:

```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev
```

On Fedora, install `gcc` and `sqlite-devel`. On macOS, install the Xcode Command Line Tools (`xcode-select --install`).

## Run locally on Linux/macOS

From the project directory:

```bash
go run ./cmd/club
```

The default database path is `./data/club.db`. The program creates the `data` directory and database automatically. Override the path with `CLUB_DB_PATH`:

```bash
CLUB_DB_PATH=./data/demo.db go run ./cmd/club
```

On the first launch with an empty database, eight fictional sample members are inserted. Sample data is not reinserted when records already exist.

## Run with Podman

Build the image:

```bash
podman build -t student-club-system:1.0 .
```

### Option A: named volume (simplest)

A named volume is persistent independently of the container. Create it once:

```bash
podman volume create student-club-data
```

Start the application:

```bash
podman run --rm -it --name student-club \
  -v student-club-data:/data \
  student-club-system:1.0
```

Use the same `student-club-data` volume every time you run the application. `--rm` removes the container after exit, not the named volume.

### Option B: bind mount (database visible in the project directory)

Create a host directory and mount it into the container:

```bash
mkdir -p data
podman run --rm -it --name student-club \
  --userns=keep-id --user "$(id -u):$(id -g)" \
  -v "$(pwd)/data:/data:Z" \
  student-club-system:1.0
```

The user mapping lets the container process write to a directory owned by your Linux user. The `:Z` suffix is useful on SELinux-enabled Linux distributions; omit `:Z` on systems that do not use SELinux.

For Docker, use the same bind mount and user mapping, replacing `podman` with `docker` and omitting `--userns=keep-id` and `:Z`:

```bash
mkdir -p data
docker run --rm -it --name student-club \
  --user "$(id -u):$(id -g)" \
  -v "$(pwd)/data:/data" \
  student-club-system:1.0
```

## Prove persistence (important for evaluation)

Using the bind-mount option above:

1. Run the container and register a fictional member, for example `STU2001`.
2. Choose **7. Exit**.
3. Confirm the host database exists: `ls -l data/club.db`.
4. The container was started with `--rm`, so it is already removed after exit. The `data/club.db` file remains on the host.
5. Start a new container with the same host directory mount and user mapping:

   ```bash
   podman run --rm -it --name student-club \
     --userns=keep-id --user "$(id -u):$(id -g)" \
     -v "$(pwd)/data:/data:Z" \
     student-club-system:1.0
   ```
6. Search for `STU2001`. The record should still exist with its saved role/status.

**Why it persists:** the database file is physically stored in the host's `./data` directory. The container can be replaced without deleting that host file. Reusing a different or empty host directory will point the application at a different database.

If you use the named-volume option instead, repeat the `podman run` command with the same `student-club-data:/data` mount. Do not remove the volume when testing persistence.

## Menu

```text
1. Register a new member
2. List all members
3. Search members
4. Assign / update a club role
5. Deactivate membership
6. View membership statistics
7. Exit
```

### Data rules

- Student ID is required and unique (case-insensitive).
- Name and department are required.
- Academic year must be between 1 and 6.
- New registrations receive the `Member` role and `Active` status by default.
- Roles are restricted to the six roles shown in the role assignment menu.
- Deactivation is a status update; it does not delete a record. Deactivating an already inactive member is safe to repeat.
- Search is case-insensitive and checks student ID, name, and department.

## Project structure

```text
student-club-system/
├── cmd/club/main.go             # Application entry point and signal handling
├── internal/cli/app.go          # Terminal menu and user interaction
├── internal/member/
│   ├── member.go                # Domain types and role/status definitions
│   ├── errors.go                # Domain errors
│   └── service.go               # Validation and membership business rules
├── internal/storage/sqlite.go   # SQLite schema, queries, and sample data
├── data/                        # Host-mounted persistent database directory
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## Tests and static checks

```bash
go test ./...
go vet ./...
go build ./cmd/club
```

The automated tests cover registration, duplicate IDs, invalid input, listing/search, role assignment, deactivation without deletion, and grouped statistics.

## Reset sample data

Stop the application first, then remove the database files:

```bash
rm -f data/club.db data/club.db-shm data/club.db-wal
```

The next launch creates a fresh database and inserts the fictional sample records again. This permanently removes the existing local project data, so only do this when you intend to reset it.
