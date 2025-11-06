# Main file for laforge utility
"""
This utility is meant to be called by a coding agent to interact with the Unit
database and artifact registry, and manipulate the current working directory.

What is a Unit? A Unit has a unique identifier, a description, and some number
of artifacts (files) that capture the requirements/specification, design
decisions, implementation, and test cases for a small functional unit within the
project. Units can depend on other Units, forming an acyclic graph of
dependencies. Once finalized, that Unit is immutable; however, a new version can
be created that supercedes the previous version.

Example Unit types for a Golang project might be:
- foo.go file along with foo_test.go and a foo.go.spec
- go.mod file along with go.sum, on which the go files that use the packages depend
- Makefile with targets, which depend on the implementations of those targets
- README.md file outlining the project goals that implementation references
- AGENTS.md containing the agent instructions, added as a dependency of every Unit

The artifact registry is a directory outside the working directory specified in
the .laforge configuration file. When in a working directory, the .laforge-unit
file contains the Unit ID of the current Unit being worked on.

The following commands are supported:

laforge init - Creates a new root Unit in the project and adds it to the artifact registry.
laforge create <unit_name> <description> - Creates a new Unit with a unique ID.
laforge add <files...> - Updates the current Unit with new files, adding them to the artifact registry.
laforge add-dep <unit_id> - Adds a dependency to the current Unit with the given unit ID.
laforge rm-dep <unit_id> - Removes a dependency from the current Unit with the given unit ID.
laforge checkout <unit_id> - Updates local working directory to the contents of a given unit by copying in all files from all the dependencies.
laforge tree - Prints the current Unit and all its dependencies in a tree format.

The artifact registry is a directory with subdirectories for each unit, e.g.
/path/to/artifact_registry/unit_id.

The Unit database is a simple sqlite3 database stored in the same directory as
the artifact registry with tables for Units, their dependencies and files.
"""

import json
import os
import shutil
import sqlite3
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

PLAN_TEMPLATE = """
# Plan file for unit `{unit_id}`

## Description
{description}

## Acceptance Criteria
{acceptance_criteria}

## Unit dependencies
```
{unit_id} [Current Unit]
{unit_dependencies}
```

## Plan
TODO: Agent will fill in this section.
"""


class LaforgeConfig:
    """Manages .laforge configuration file"""

    def __init__(self, config_path: str = ".laforge"):
        self.config_path = Path(config_path)

    def load(self) -> Optional[Dict]:
        """Load configuration from .laforge file"""
        if not self.config_path.exists():
            return None
        try:
            with open(self.config_path, "r") as f:
                return json.load(f)
        except Exception as e:
            print(f"Error loading config: {e}", file=sys.stderr)
            return None

    def save(self, config: Dict) -> bool:
        """Save configuration to .laforge file"""
        try:
            with open(self.config_path, "w") as f:
                json.dump(config, f, indent=2)
            return True
        except Exception as e:
            print(f"Error saving config: {e}", file=sys.stderr)
            return False

    def get_registry_path(self) -> Optional[Path]:
        """Get artifact registry path from config"""
        config = self.load()
        if config and "artifact_registry" in config:
            return Path(config["artifact_registry"])
        return None

    def get_default_units(self) -> List[str]:
        """Get default units from config"""
        config = self.load()
        if config and "default_units" in config:
            return config["default_units"]
        return []


class LaforgeUnit:
    """Manages .laforge-unit file for current unit"""

    def __init__(self, unit_file: str = ".laforge-unit"):
        self.unit_file = Path(unit_file)

    def load(self) -> Optional[Dict]:
        """Load current unit info"""
        if not self.unit_file.exists():
            return None
        try:
            with open(self.unit_file, "r") as f:
                return json.load(f)
        except Exception as e:
            print(f"Error loading unit: {e}", file=sys.stderr)
            return None

    def save(self, unit_id: str, path: Path | None = None) -> bool:
        """Save current unit info"""
        try:
            with open(path / ".laforge-unit" if path else self.unit_file, "w") as f:
                json.dump({"unit_id": unit_id}, f, indent=2)
            return True
        except Exception as e:
            print(f"Error saving unit: {e}", file=sys.stderr)
            return False


class LaforgeDB:
    """Manages SQLite database for Units and dependencies"""

    def __init__(self, db_path: str):
        self.db_path = Path(db_path)
        self._init_db()

    def _init_db(self):
        """Initialize database schema if needed"""
        conn = sqlite3.connect(str(self.db_path))
        cursor = conn.cursor()

        # Create Units table
        _ = cursor.execute("""
            CREATE TABLE IF NOT EXISTS units (
                unit_id TEXT PRIMARY KEY,
                description TEXT,
                created_at TEXT,
                finalized BOOLEAN DEFAULT 0
            )
        """)

        # Create Dependencies table (unit to unit)
        _ = cursor.execute("""
            CREATE TABLE IF NOT EXISTS dependencies (
                unit_id TEXT,
                depends_on_unit_id TEXT,
                PRIMARY KEY (unit_id, depends_on_unit_id),
                FOREIGN KEY (unit_id) REFERENCES units(unit_id),
                FOREIGN KEY (depends_on_unit_id) REFERENCES units(unit_id)
            )
        """)

        # Create Supercedes table (unit to unit)
        _ = cursor.execute("""
            CREATE TABLE IF NOT EXISTS supercedes (
                unit_id TEXT,
                supercedes_unit_id TEXT,
                PRIMARY KEY (unit_id, supercedes_unit_id),
                FOREIGN KEY (unit_id) REFERENCES units(unit_id),
                FOREIGN KEY (supercedes_unit_id) REFERENCES units(unit_id)
            )
        """)

        # Create Artifacts table
        _ = cursor.execute("""
            CREATE TABLE IF NOT EXISTS files (
                unit_id TEXT,
                file_path TEXT,
                PRIMARY KEY (unit_id, file_path),
                FOREIGN KEY (unit_id) REFERENCES units(unit_id)
            )
        """)

        conn.commit()
        conn.close()

        print(f"Successfully opened {self.db_path}")

    def get_connection(self):
        """Get database connection"""
        return sqlite3.connect(str(self.db_path))

    def create_unit(self, unit_id: str, description: str):
        """Create a new Unit"""
        conn = self.get_connection()
        cursor = conn.cursor()
        _ = cursor.execute(
            """
            INSERT INTO units (unit_id, description, created_at)
            VALUES (?, ?, ?)
        """,
            (unit_id, description, datetime.now().isoformat()),
        )
        conn.commit()
        conn.close()

    def get_unit(self, unit_id: str) -> Optional[Dict]:
        """Get unit info"""
        conn = self.get_connection()
        cursor = conn.cursor()
        _ = cursor.execute(
            "SELECT unit_id, description, created_at, finalized FROM units WHERE unit_id = ?",
            (unit_id,),
        )
        row = cursor.fetchone()
        conn.close()

        if row:
            return {
                "unit_id": row[0],
                "description": row[1],
                "created_at": row[2],
                "finalized": row[3],
            }
        return None

    def get_unit_dependencies(self, unit_id: str) -> List[Dict]:
        """Get all unit dependencies for a unit"""
        conn = self.get_connection()
        cursor = conn.cursor()
        cursor.execute(
            """
            SELECT depends_on_unit_id FROM dependencies WHERE unit_id = ?
        """,
            (unit_id,),
        )
        rows = cursor.fetchall()
        conn.close()

        return [{"unit_id": row[0]} for row in rows]

    def add_dependency(self, unit_id: str, depends_on_unit_id: str) -> bool:
        """Add or replace a unit dependency"""
        conn = self.get_connection()
        cursor = conn.cursor()

        try:
            _ = cursor.execute(
                """
                INSERT OR REPLACE INTO dependencies (unit_id, depends_on_unit_id)
                VALUES (?, ?)
            """,
                (unit_id, depends_on_unit_id),
            )
            conn.commit()
            return True
        except Exception as e:
            print(f"Error adding dependency: {e}", file=sys.stderr)
            return False
        finally:
            conn.close()

    def remove_dependency(self, unit_id: str, depends_on_unit_id: str) -> bool:
        """Remove a unit dependency"""
        conn = self.get_connection()
        cursor = conn.cursor()

        try:
            _ = cursor.execute(
                """
                DELETE FROM dependencies WHERE unit_id = ? AND depends_on_unit_id = ?
            """,
                (unit_id, depends_on_unit_id),
            )
            if cursor.rowcount == 0:
                print(
                    f"Error: Dependency on {depends_on_unit_id} not found",
                    file=sys.stderr,
                )
                return False
            conn.commit()
            return True
        except Exception as e:
            print(f"Error removing dependency: {e}", file=sys.stderr)
            return False
        finally:
            conn.close()

    def associate_file(self, unit_id: str, file_path: str):
        """Create a new artifact for a unit"""
        conn = self.get_connection()
        cursor = conn.cursor()
        _ = cursor.execute(
            """
            INSERT OR REPLACE INTO files (unit_id, file_path)
            VALUES (?, ?)
        """,
            (unit_id, file_path),
        )
        conn.commit()
        conn.close()

    def get_unit_files(self, unit_id: str) -> set[str]:
        """Get all artifacts for a unit"""
        conn = self.get_connection()
        cursor = conn.cursor()
        _ = cursor.execute(
            """
            SELECT file_path FROM files WHERE unit_id = ?
            """,
            (unit_id,),
        )
        rows = cursor.fetchall()
        conn.close()

        return set(row[0] for row in rows)

    def get_all_files(
        self, unit_id: str, visited: Optional[set[str]] = None
    ) -> tuple[set[str], set[str]]:
        """Get all artifacts for a unit and all its dependencies recursively"""
        if visited is None:
            visited = set()

        if unit_id in visited:
            return set(), set()

        visited.add(unit_id)
        files = self.get_unit_files(unit_id)
        units = set([unit_id])

        # Recursively get artifacts from dependencies
        dependencies = self.get_unit_dependencies(unit_id)
        for dep in dependencies:
            dep_files, dep_units = self.get_all_files(dep["unit_id"], visited)
            files.update(dep_files)
            units.update(dep_units)

        return files, units

    def get_root_units(self) -> List[Dict]:
        """Get all root units (units that do not appear as depends_on_unit_id)"""
        conn = self.get_connection()
        cursor = conn.cursor()
        _ = cursor.execute(
            """
            SELECT u.unit_id, u.description FROM units u
            WHERE u.unit_id NOT IN (
                SELECT DISTINCT depends_on_unit_id FROM dependencies
            )
            ORDER BY u.created_at
        """
        )
        rows = cursor.fetchall()
        conn.close()

        return [
            {
                "unit_id": row[0],
                "description": row[1],
            }
            for row in rows
        ]


class LaforgeRegistry:
    """Manages artifact registry"""

    def __init__(self, registry_path: Path):
        self.registry_path = Path(registry_path)

    def ensure_exists(self) -> bool:
        """Ensure registry directory exists"""
        try:
            self.registry_path.mkdir(parents=True, exist_ok=True)
            return True
        except Exception as e:
            print(f"Error creating registry: {e}", file=sys.stderr)
            return False

    def store_file(
        self, unit_id: str, file_path: str, contents: str | None = None
    ) -> bool:
        """Store file in artifact registry"""
        artifact_dir = self.registry_path / unit_id
        try:
            artifact_dir.mkdir(parents=True, exist_ok=True)

            src = Path(file_path)
            dst_path = artifact_dir / src.parent
            dst_path.mkdir(parents=True, exist_ok=True)
            dst = dst_path / src.name
            if contents is not None:
                with open(dst, "w") as f:
                    f.write(contents)
            elif src.exists():
                if src.is_file():
                    shutil.copy2(src, dst)
                else:
                    raise ValueError(f"Cannot store non-file {src}")

            return True
        except Exception as e:
            print(f"Error storing file: {e}", file=sys.stderr)
            return False

    def restore_files(
        self, unit_id: str, dest_dir: Path, include_internal: bool
    ) -> set[str]:
        """Restore artifact files to destination"""
        artifact_dir = self.registry_path / unit_id
        if not artifact_dir.exists():
            # Nothing to restore
            return set()

        try:
            restored_files = set()
            dest_dir.mkdir(parents=True, exist_ok=True)

            # Recursively traverse artifact_dir using os.walk for compatibility
            artifact_dir_str = str(artifact_dir)
            for root, dirs, files in os.walk(artifact_dir_str):
                root_path = Path(root)
                # Get relative path from artifact_dir
                rel_path = root_path.relative_to(artifact_dir)

                # Handle files
                for file_name in files:
                    file_rel_path = (
                        rel_path / file_name
                        if rel_path != Path(".")
                        else Path(file_name)
                    )

                    # Check if internal file should be skipped (only if at root of artifact directory)
                    is_at_root = rel_path == Path(".")
                    if (
                        not include_internal
                        and is_at_root
                        and file_rel_path.name.lower()
                        in [
                            "plan.md",
                            "review.md",
                            "wants.md",
                        ]
                    ):
                        print(f"Skipping {file_rel_path} in {unit_id}")  # donotcheckin
                        continue

                    src_file = root_path / file_name
                    dst_file = dest_dir / file_rel_path
                    dst_file.parent.mkdir(parents=True, exist_ok=True)

                    print(f"Restoring {file_rel_path} in {unit_id}")  # donotcheckin
                    shutil.copy2(src_file, dst_file)
                    restored_files.add(str(file_rel_path))

            return restored_files
        except Exception as e:
            raise Exception(f"Error restoring files: {e}")


def print_unit_tree(
    db: LaforgeDB, unit_id: str, prefix: str = "", is_last: bool = True
) -> str:
    """Recursively print unit tree"""
    unit = db.get_unit(unit_id)
    if not unit:
        return ""

    # Print current unit
    connector = "└── " if is_last else "├── "
    ret = f"{prefix}{connector}{unit['unit_id']}: {unit['description']}\n"

    # Get units that this unit depends on
    dependencies = db.get_unit_dependencies(unit_id)

    # Print wants
    extension = "    " if is_last else "|   "
    total_items = len(dependencies)

    # Print each dependency
    for i, dependency in enumerate(dependencies):
        is_last_dependency = i == total_items - 1
        ret += print_unit_tree(
            db, dependency["unit_id"], prefix + extension, is_last_dependency
        )

    return ret


def cmd_init(args: List[str]):
    """laforge init - Create root Unit"""
    config = LaforgeConfig().load()
    if not config:
        print(
            "Error: .laforge config file not found. Initialize with registry path first.",
            file=sys.stderr,
        )
        return False

    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Create root unit
    db.create_unit("root", "Root Unit")

    # Save as current unit
    if not LaforgeUnit().save("root"):
        return False

    print("Created unit: 'root'")
    return True


def cmd_create(args: List[str]):
    """laforge create <unit_name> <description> <acceptance_criteria> [<dependency> ...]"""
    if len(args) < 3:
        print(
            "Usage: laforge create <unit_id> <description> <acceptance_criteria> [<dependency> ...]",
            file=sys.stderr,
        )
        return False

    unit_id = args[0]
    description = args[1]
    acceptance_criteria = args[2]
    dependencies = args[3:]

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))
    registry = LaforgeRegistry(registry_path)

    if not registry.ensure_exists():
        return False

    existing = db.get_unit(unit_id)
    if existing is not None:
        print(f"Unit {unit_id} already exists", file=sys.stderr)
        return False

    # Create new unit with description
    db.create_unit(unit_id, description)

    # Add initial dependencies
    for default_unit_id in LaforgeConfig().get_default_units():
        if not db.add_dependency(unit_id, default_unit_id):
            print(
                f"Failed to add dependency '{default_unit_id}' for unit {unit_id}",
                file=sys.stderr,
            )
            return False

    for dependency in dependencies:
        if not db.add_dependency(unit_id, dependency):
            print(
                f"Failed to add dependency '{dependency}' for unit {unit_id}",
                file=sys.stderr,
            )
            return False

    # Create initial PLAN.md file
    dependencies = db.get_unit_dependencies(unit_id)
    total_items = len(dependencies)
    unit_dependencies = "".join(
        [
            print_unit_tree(db, dependency["unit_id"], "", i == total_items - 1)
            for i, dependency in enumerate(dependencies)
        ]
    )
    registry.store_file(
        unit_id,
        "PLAN.md",
        PLAN_TEMPLATE.format(
            unit_id=unit_id,
            description=description,
            acceptance_criteria=acceptance_criteria,
            unit_dependencies=unit_dependencies,
        ),
    )
    db.associate_file(unit_id, "PLAN.md")

    print(f"Created unit: '{unit_id}'")
    return True


def cmd_add(args: List[str]):
    """laforge add <files...>"""
    if len(args) < 1:
        print("Usage: laforge add <files...>", file=sys.stderr)
        return False

    files = args

    # Get current unit
    unit_info = LaforgeUnit().load()
    if not unit_info:
        print("Error: No current unit. Run 'laforge init' first.", file=sys.stderr)
        return False

    current_unit_id = unit_info["unit_id"]

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))
    registry = LaforgeRegistry(registry_path)

    if not registry.ensure_exists():
        return False

    for file_path in files:
        # Create new artifact for current unit
        db.associate_file(current_unit_id, file_path)

        # Store file
        if not registry.store_file(current_unit_id, file_path):
            return False

    print(f"Added files to unit: '{current_unit_id}'")
    return True


def cmd_add_dep(args: List[str]):
    """laforge add-dep <unit_id>"""
    if len(args) < 1:
        print(
            "Usage: laforge add-dep <unit_id>",
            file=sys.stderr,
        )
        return False

    depends_on_unit_id = args[0]

    # Get current unit
    unit_info = LaforgeUnit().load()
    if not unit_info:
        print("Error: No current unit. Run 'laforge init' first.", file=sys.stderr)
        return False

    current_unit_id = unit_info["unit_id"]

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Verify unit exists
    unit = db.get_unit(depends_on_unit_id)
    if not unit:
        print(f"Error: Unit {depends_on_unit_id} not found", file=sys.stderr)
        return False

    # Add dependency
    if not db.add_dependency(current_unit_id, depends_on_unit_id):
        return False

    print(f"Added dependency '{depends_on_unit_id}' to unit '{current_unit_id}'")

    return True


def cmd_rm_dep(args: List[str]):
    """laforge rm-dep <unit_id>"""
    if len(args) < 1:
        print("Usage: laforge rm-dep <unit_id>", file=sys.stderr)
        return False

    depends_on_unit_id = args[0]

    # Get current unit
    unit_info = LaforgeUnit().load()
    if not unit_info:
        print("Error: No current unit. Run 'laforge init' first.", file=sys.stderr)
        return False

    current_unit_id = unit_info["unit_id"]

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Remove dependency
    if not db.remove_dependency(current_unit_id, depends_on_unit_id):
        return False

    print(f"Removed dependency: {depends_on_unit_id}")
    return True


def cmd_checkout(args: List[str]):
    """laforge checkout <unit_id> [path]"""
    if len(args) < 1:
        print("Usage: laforge checkout <unit_id> [path]", file=sys.stderr)
        return False

    unit_id = args[0]
    work_dir = Path(args[1]) if len(args) > 1 else Path(".")

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))
    registry = LaforgeRegistry(registry_path)

    # Get unit
    unit = db.get_unit(unit_id)
    if not unit:
        print(f"Error: Unit '{unit_id}' not found", file=sys.stderr)
        return False

    # Collect all files that will be restored
    _, tree_units = db.get_all_files(unit_id)

    work_dir.mkdir(parents=True, exist_ok=True)

    # Copy .laforge file to working directory
    if not (work_dir / ".laforge").exists():
        shutil.copy(".laforge", str(work_dir / ".laforge"))

    # Restore all artifact files to current directory
    files_to_keep = set()
    for tree_unit in tree_units:
        print(f" -> Restoring files from {tree_unit}...")
        restored_files = registry.restore_files(
            tree_unit, work_dir, include_internal=tree_unit == unit_id
        )
        print(f"  restored files {restored_files}")  # donotcheckin
        files_to_keep |= restored_files

    # Delete non-dotfiles not in the checkout
    # Recursively traverse work_dir to find files/directories to delete
    try:
        work_dir_str = str(work_dir)
        # Traverse bottom-up (post-order) so we can delete directories after their contents
        for root, dirs, files in os.walk(work_dir_str, topdown=False):
            root_path = Path(root)

            # Handle files
            for file_name in files:
                file_path = root_path / file_name
                rel_path = file_path.relative_to(work_dir)

                # Skip dotfiles
                if rel_path.parts[0].startswith("."):
                    continue

                # Skip files that are in the checkout
                if str(rel_path) in files_to_keep:
                    continue

                # Delete this file
                print(f" - Deleting {rel_path}...")
                file_path.unlink()

            # Handle directories (processed after files in bottom-up traversal)
            if root_path != work_dir:  # Don't delete the work_dir itself
                rel_path = root_path.relative_to(work_dir)

                # Skip dotdirectories
                if rel_path.parts[0].startswith("."):
                    continue

                # Try to delete directory (will only succeed if empty)
                # This ensures we don't delete directories that contain files we want to keep
                try:
                    root_path.rmdir()
                    print(f" - Deleting {rel_path}...")
                except OSError:
                    # Directory not empty, skip (contains files that should be kept)
                    pass

        # Handle top-level items that might not be directories
        for item in work_dir.iterdir():
            # Skip dotfiles and dotdirectories
            if item.name.startswith("."):
                continue

            # Get relative path for comparison
            rel_path = item.relative_to(work_dir)

            # Skip items that are in the checkout
            if str(rel_path) in files_to_keep:
                continue

            # Delete this item (shouldn't happen often since walk covers most cases)
            if item.is_file():
                print(f" - Deleting {rel_path}...")
                item.unlink()
            elif item.is_dir():
                # Only delete if directory is empty
                try:
                    item.rmdir()
                    print(f" - Deleting {rel_path}...")
                except OSError:
                    # Directory not empty, skip (contains files that should be kept)
                    pass
    except Exception as e:
        print(f"Error cleaning working directory: {e}", file=sys.stderr)
        return False

    # Update current unit
    if not LaforgeUnit().save(unit_id, work_dir):
        return False

    print(f"Checked out unit: '{unit_id}'")
    return True


def cmd_tree(args: List[str]):
    """laforge tree - Print Units in tree format starting from current unit"""
    # Get current unit
    unit_info = LaforgeUnit().load()
    if not unit_info:
        print("Error: No current unit. Run 'laforge init' first.", file=sys.stderr)
        return False

    current_unit_id = unit_info["unit_id"]

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Get current unit
    current_unit = db.get_unit(current_unit_id)
    if not current_unit:
        print(f"Error: Unit {current_unit_id} not found", file=sys.stderr)
        return False

    # Print current unit and its tree
    print(f"{current_unit_id}: {current_unit['description']}")

    # Get dependencies
    dependencies = db.get_unit_dependencies(current_unit_id)

    total_items = len(dependencies)

    # Print each dependency
    for i, dependency in enumerate(dependencies):
        is_last_dependency = i == total_items - 1
        print(
            print_unit_tree(db, dependency["unit_id"], "", is_last_dependency).strip(
                "\n"
            )
        )

    return True


def main():
    """Main entry point"""
    if len(sys.argv) < 2:
        print("Usage: laforge <command> [args...]", file=sys.stderr)
        print(
            "Commands: init, create, add, add-dep, rm-dep, checkout, tree",
            file=sys.stderr,
        )
        sys.exit(1)

    command = sys.argv[1]
    args = sys.argv[2:]

    commands = {
        "init": cmd_init,
        "create": cmd_create,
        "add": cmd_add,
        "add-dep": cmd_add_dep,
        "rm-dep": cmd_rm_dep,
        "checkout": cmd_checkout,
        "tree": cmd_tree,
    }

    if command not in commands:
        print(f"Unknown command: {command}", file=sys.stderr)
        sys.exit(1)

    success = commands[command](args)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
