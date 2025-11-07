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
laforge diff - Compares the current working directory files with the artifact registry versions for the current Unit.
laforge finalize [unit_id] - Marks a Unit as finalized (immutable). If no unit_id is provided, finalizes the current Unit.
laforge apply <yaml_file> [--dry-run] - Applies operations from a YAML file to create or modify units.
laforge update <old_unit_id> <new_unit_id> [output_file] - Generates a YAML file with operations to update dependencies. For finalized units, creates new versions recursively. Default output: updates.yaml
laforge next - Lists units that are ready to work on (not finalized, all dependencies finalized).

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

import yaml

INTERNAL_FILES = [
    "plan.md",
    "review.md",
    "wants.md",
]

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
        self,
        unit_id: str,
        visited: Optional[set[str]] = None,
        include_internal=True,
    ) -> tuple[set[str], set[str]]:
        """Get all artifacts for a unit and all its dependencies recursively"""
        if visited is None:
            visited = set()

        if unit_id in visited:
            return set(), set()

        visited.add(unit_id)
        files = set(
            [
                file_path
                for file_path in self.get_unit_files(unit_id)
                if include_internal or file_path.lower() not in INTERNAL_FILES
            ]
        )
        units = set([unit_id])

        # Recursively get artifacts from dependencies
        dependencies = self.get_unit_dependencies(unit_id)
        for dep in dependencies:
            dep_files, dep_units = self.get_all_files(
                dep["unit_id"], visited, include_internal=False
            )
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

    def get_dependents(self, unit_id: str) -> List[str]:
        """Get all units that depend on the given unit"""
        conn = self.get_connection()
        cursor = conn.cursor()
        cursor.execute(
            """
            SELECT unit_id FROM dependencies WHERE depends_on_unit_id = ?
            """,
            (unit_id,),
        )
        rows = cursor.fetchall()
        conn.close()

        return [row[0] for row in rows]

    def find_available_version(self, base_unit_id: str) -> str:
        """Find an available versioned unit ID (e.g., base-v2, base-v3, etc.)"""
        # First check if the base ID itself exists
        if not self.get_unit(base_unit_id):
            return base_unit_id

        # Try versioned IDs
        version = 2
        while True:
            versioned_id = f"{base_unit_id}-v{version}"
            if not self.get_unit(versioned_id):
                return versioned_id
            version += 1

    def copy_unit(
        self, old_unit_id: str, new_unit_id: str, registry: "LaforgeRegistry"
    ) -> bool:
        """Create a copy of a unit with a new ID, copying all files and metadata"""
        # Get the old unit
        old_unit = self.get_unit(old_unit_id)
        if not old_unit:
            print(f"Error: Unit '{old_unit_id}' not found", file=sys.stderr)
            return False

        # Create the new unit
        self.create_unit(new_unit_id, old_unit["description"])

        # Copy all dependencies
        dependencies = self.get_unit_dependencies(old_unit_id)
        for dep in dependencies:
            self.add_dependency(new_unit_id, dep["unit_id"])

        # Copy all files
        files = self.get_unit_files(old_unit_id)
        for file_path in files:
            # Copy file in registry
            old_artifact_path = registry.registry_path / old_unit_id / file_path
            if old_artifact_path.exists():
                # Read contents from old location
                with open(old_artifact_path, "rb") as f:
                    contents = f.read()

                # Write to new location
                new_artifact_path = registry.registry_path / new_unit_id / file_path
                new_artifact_path.parent.mkdir(parents=True, exist_ok=True)
                with open(new_artifact_path, "wb") as f:
                    f.write(contents)

                # Associate with new unit
                self.associate_file(new_unit_id, file_path)

        # Mark that new unit supersedes old unit
        conn = self.get_connection()
        cursor = conn.cursor()
        try:
            cursor.execute(
                """
                INSERT INTO supercedes (unit_id, supercedes_unit_id)
                VALUES (?, ?)
                """,
                (new_unit_id, old_unit_id),
            )
            conn.commit()
        except Exception as e:
            print(f"Error marking supersedes: {e}", file=sys.stderr)
            return False
        finally:
            conn.close()

        return True


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
                        and file_rel_path.name.lower() in INTERNAL_FILES
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

    # Print current unit with status indicator
    connector = "└── " if is_last else "├── "
    status_emoji = "✅" if unit["finalized"] else "🚧"
    ret = (
        f"{prefix}{connector}{status_emoji} {unit['unit_id']}: {unit['description']}\n"
    )

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


def validate_yaml_operations(
    operations: List[Dict], db: LaforgeDB
) -> tuple[bool, str, dict]:
    """Validate YAML operations before execution.

    Returns (success, error_message, id_remapping)
    """
    if not isinstance(operations, list):
        return False, "YAML file must contain a list of operations", {}

    # Track units that will be created in this batch
    units_to_create = set()
    # Track ID remappings (original_id -> new_id)
    id_remapping = {}

    # First pass: collect all units that will be created
    for i, op in enumerate(operations):
        if not isinstance(op, dict):
            return False, f"Operation {i} is not a valid dictionary", {}

        if "action" not in op:
            return False, f"Operation {i} missing 'action' field", {}

        action = op["action"]

        if action == "CREATE":
            if "id" not in op:
                return False, f"CREATE operation {i} missing 'id' field", {}
            if "title" not in op:
                return False, f"CREATE operation {i} missing 'title' field", {}
            if "acceptance_criteria" not in op:
                return (
                    False,
                    f"CREATE operation {i} missing 'acceptance_criteria' field",
                    {},
                )

            original_unit_id = op["id"]
            unit_id = original_unit_id

            # Check if unit already exists in database - if so, remap to available ID
            existing = db.get_unit(unit_id)
            if existing:
                version = 2
                while True:
                    new_unit_id = f"{original_unit_id}-v{version}"
                    if (
                        not db.get_unit(new_unit_id)
                        and new_unit_id not in units_to_create
                    ):
                        unit_id = new_unit_id
                        id_remapping[original_unit_id] = new_unit_id
                        op["id"] = new_unit_id
                        print(
                            f"Unit '{original_unit_id}' already exists, remapping to '{new_unit_id}'"
                        )
                        break
                    version += 1

            # Check for duplicates in the same file
            if unit_id in units_to_create:
                return False, f"Duplicate CREATE operation for unit '{unit_id}'", {}

            units_to_create.add(unit_id)

            # Validate acceptance_criteria is a list
            if not isinstance(op["acceptance_criteria"], list):
                return (
                    False,
                    f"Unit '{unit_id}': acceptance_criteria must be a list",
                    {},
                )

            # Validate dependencies if present
            if "dependencies" in op:
                if not isinstance(op["dependencies"], list):
                    return False, f"Unit '{unit_id}': dependencies must be a list", {}

        elif action == "ADD_DEPENDENCIES":
            if "id" not in op:
                return False, f"ADD_DEPENDENCIES operation {i} missing 'id' field", {}
            if "dependencies" not in op:
                return (
                    False,
                    f"ADD_DEPENDENCIES operation {i} missing 'dependencies' field",
                    {},
                )
            if not isinstance(op["dependencies"], list):
                return (
                    False,
                    f"ADD_DEPENDENCIES operation {i}: dependencies must be a list",
                    {},
                )

        elif action == "REMOVE_DEPENDENCIES":
            if "id" not in op:
                return (
                    False,
                    f"REMOVE_DEPENDENCIES operation {i} missing 'id' field",
                    {},
                )
            if "dependencies" not in op:
                return (
                    False,
                    f"REMOVE_DEPENDENCIES operation {i} missing 'dependencies' field",
                    {},
                )
            if not isinstance(op["dependencies"], list):
                return (
                    False,
                    f"REMOVE_DEPENDENCIES operation {i}: dependencies must be a list",
                    {},
                )

        elif action == "COPY_UNIT":
            if "source_id" not in op:
                return False, f"COPY_UNIT operation {i} missing 'source_id' field", {}
            if "new_id" not in op:
                return False, f"COPY_UNIT operation {i} missing 'new_id' field", {}
            if "dependencies" not in op:
                return (
                    False,
                    f"COPY_UNIT operation {i} missing 'dependencies' field",
                    {},
                )
            if not isinstance(op["dependencies"], list):
                return (
                    False,
                    f"COPY_UNIT operation {i}: dependencies must be a list",
                    {},
                )

            original_new_id = op["new_id"]
            new_id = original_new_id

            # Check if new_id already exists in database - if so, remap to available ID
            existing = db.get_unit(new_id)
            if existing:
                version = 2
                while True:
                    versioned_id = f"{original_new_id}-v{version}"
                    if (
                        not db.get_unit(versioned_id)
                        and versioned_id not in units_to_create
                    ):
                        new_id = versioned_id
                        id_remapping[original_new_id] = new_id
                        op["new_id"] = new_id
                        print(
                            f"Unit '{original_new_id}' already exists, remapping to '{new_id}'"
                        )
                        break
                    version += 1

            # Check for duplicates in the same file
            if new_id in units_to_create:
                return False, f"Duplicate COPY_UNIT operation for unit '{new_id}'", {}

            units_to_create.add(new_id)

        else:
            return False, f"Unknown action '{action}' in operation {i}", {}

    # Second pass: apply ID remapping to dependencies and validate all referenced units exist or will be created
    for i, op in enumerate(operations):
        action = op["action"]

        if action == "CREATE":
            unit_id = op["id"]
            dependencies = op.get("dependencies", [])

            # Apply ID remapping to dependencies
            remapped_dependencies = []
            for dep_id in dependencies:
                remapped_dep_id = id_remapping.get(dep_id, dep_id)
                remapped_dependencies.append(remapped_dep_id)

                # Check if dependency exists in DB or will be created
                if remapped_dep_id not in units_to_create and not db.get_unit(
                    remapped_dep_id
                ):
                    return (
                        False,
                        f"Unit '{unit_id}' depends on '{remapped_dep_id}' which does not exist",
                        {},
                    )

            # Update dependencies with remapped IDs
            if dependencies:
                op["dependencies"] = remapped_dependencies

        elif action == "COPY_UNIT":
            source_id = op["source_id"]
            new_id = op["new_id"]
            dependencies = op.get("dependencies", [])

            # Verify source unit exists
            if not db.get_unit(source_id):
                return False, f"COPY_UNIT source unit '{source_id}' does not exist", {}

            # Apply ID remapping to dependencies
            remapped_dependencies = []
            for dep_id in dependencies:
                remapped_dep_id = id_remapping.get(dep_id, dep_id)
                remapped_dependencies.append(remapped_dep_id)

                # Check if dependency exists in DB or will be created
                if remapped_dep_id not in units_to_create and not db.get_unit(
                    remapped_dep_id
                ):
                    return (
                        False,
                        f"Unit '{new_id}' depends on '{remapped_dep_id}' which does not exist",
                        {},
                    )

            # Update dependencies with remapped IDs
            if dependencies:
                op["dependencies"] = remapped_dependencies

        elif action in ["ADD_DEPENDENCIES", "REMOVE_DEPENDENCIES"]:
            original_unit_id = op["id"]
            unit_id = id_remapping.get(original_unit_id, original_unit_id)

            # Update the operation's ID if it was remapped
            if original_unit_id in id_remapping:
                op["id"] = unit_id

            # Check if the unit exists or will be created
            if unit_id not in units_to_create and not db.get_unit(unit_id):
                return (
                    False,
                    f"{action} operation for unit '{unit_id}' which does not exist",
                    {},
                )

            # Apply ID remapping to dependencies and validate
            dependencies = op.get("dependencies", [])
            remapped_dependencies = []
            for dep_id in dependencies:
                remapped_dep_id = id_remapping.get(dep_id, dep_id)
                remapped_dependencies.append(remapped_dep_id)

                # For ADD_DEPENDENCIES, validate dependency units exist
                if action == "ADD_DEPENDENCIES":
                    if remapped_dep_id not in units_to_create and not db.get_unit(
                        remapped_dep_id
                    ):
                        return (
                            False,
                            f"Cannot add dependency '{remapped_dep_id}' to unit '{unit_id}': dependency does not exist",
                            {},
                        )

            # Update dependencies with remapped IDs
            if dependencies:
                op["dependencies"] = remapped_dependencies

    return True, "", id_remapping


def execute_yaml_operations(
    operations: List[Dict], db: LaforgeDB, registry: LaforgeRegistry
) -> bool:
    """Execute validated YAML operations.

    Assumes operations have already been validated.
    """
    for op in operations:
        action = op["action"]

        if action == "COPY_UNIT":
            source_id = op["source_id"]
            new_id = op["new_id"]
            dependencies = op["dependencies"]

            # Copy the unit
            if not db.copy_unit(source_id, new_id, registry):
                print(
                    f"Warning: Failed to copy unit '{source_id}' to '{new_id}'",
                    file=sys.stderr,
                )
                continue

            print(f"Copied unit: '{source_id}' -> '{new_id}'")

            # Now update the dependencies to match what was specified
            # First, remove all existing dependencies
            existing_deps = db.get_unit_dependencies(new_id)
            for dep in existing_deps:
                db.remove_dependency(new_id, dep["unit_id"])

            # Then add the new dependencies
            for dep_id in dependencies:
                if not db.add_dependency(new_id, dep_id):
                    print(
                        f"Warning: Failed to add dependency '{dep_id}' to unit '{new_id}'",
                        file=sys.stderr,
                    )

        elif action == "CREATE":
            unit_id = op["id"]
            title = op["title"]
            acceptance_criteria = op["acceptance_criteria"]
            dependencies = op.get("dependencies", [])

            # Format acceptance criteria as a string
            criteria_text = "\n".join(
                f"- {criterion}" for criterion in acceptance_criteria
            )

            # Create the unit
            db.create_unit(unit_id, title)
            print(f"Created unit: '{unit_id}'")

            # Add default dependencies
            for default_unit_id in LaforgeConfig().get_default_units():
                if not db.add_dependency(unit_id, default_unit_id):
                    print(
                        f"Warning: Failed to add default dependency '{default_unit_id}' to unit '{unit_id}'",
                        file=sys.stderr,
                    )

            # Add specified dependencies
            for dep_id in dependencies:
                if not db.add_dependency(unit_id, dep_id):
                    print(
                        f"Warning: Failed to add dependency '{dep_id}' to unit '{unit_id}'",
                        file=sys.stderr,
                    )

            # Create PLAN.md file
            deps = db.get_unit_dependencies(unit_id)
            total_items = len(deps)
            unit_dependencies = "".join(
                [
                    print_unit_tree(db, dep["unit_id"], "", i == total_items - 1)
                    for i, dep in enumerate(deps)
                ]
            )

            registry.store_file(
                unit_id,
                "PLAN.md",
                PLAN_TEMPLATE.format(
                    unit_id=unit_id,
                    description=title,
                    acceptance_criteria=criteria_text,
                    unit_dependencies=unit_dependencies,
                ),
            )
            db.associate_file(unit_id, "PLAN.md")

        elif action == "ADD_DEPENDENCIES":
            unit_id = op["id"]
            dependencies = op["dependencies"]

            for dep_id in dependencies:
                if db.add_dependency(unit_id, dep_id):
                    print(f"Added dependency '{dep_id}' to unit '{unit_id}'")
                else:
                    print(
                        f"Warning: Failed to add dependency '{dep_id}' to unit '{unit_id}'",
                        file=sys.stderr,
                    )

        elif action == "REMOVE_DEPENDENCIES":
            unit_id = op["id"]
            dependencies = op["dependencies"]

            for dep_id in dependencies:
                if db.remove_dependency(unit_id, dep_id):
                    print(f"Removed dependency '{dep_id}' from unit '{unit_id}'")
                else:
                    print(
                        f"Warning: Failed to remove dependency '{dep_id}' from unit '{unit_id}'",
                        file=sys.stderr,
                    )

    return True


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

    # Print current unit and its tree with status
    status_emoji = "✅" if current_unit["finalized"] else "🚧"
    print(f"{status_emoji} {current_unit_id}: {current_unit['description']}")

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


def cmd_diff(args: List[str]):
    """laforge diff - Compare current directory files with artifact registry versions"""
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

    # Get current unit
    current_unit = db.get_unit(current_unit_id)
    if not current_unit:
        print(f"Error: Unit {current_unit_id} not found", file=sys.stderr)
        return False

    print(f"Comparing working directory with unit '{current_unit_id}'...\n")

    # Get all files from the current unit (not dependencies)
    unit_files = db.get_unit_files(current_unit_id)

    # Get all files from dependencies to exclude them from new file detection
    all_files, _ = db.get_all_files(current_unit_id)
    dependency_files = all_files - unit_files  # Files from dependencies only

    # Track file changes
    created_files = []
    modified_files = []
    deleted_files = []
    unchanged_files = []

    # Find all non-hidden files in working directory
    working_files = set()
    work_dir = Path(".")
    for root, dirs, files in os.walk(work_dir):
        # Skip hidden directories
        dirs[:] = [d for d in dirs if not d.startswith(".")]

        root_path = Path(root)
        for file_name in files:
            file_path = root_path / file_name
            rel_path = file_path.relative_to(work_dir)

            # Skip hidden files
            if any(part.startswith(".") for part in rel_path.parts):
                continue

            working_files.add(str(rel_path))

    # Check tracked files
    for file_path in sorted(unit_files):
        artifact_path = registry.registry_path / current_unit_id / file_path
        working_path = Path(file_path)

        # Check if file exists in both locations
        if not artifact_path.exists():
            print(f"⚠️  {file_path}: Missing from artifact registry")
            continue

        if not working_path.exists():
            print(f"🗑️  {file_path}: Deleted from working directory")
            deleted_files.append(file_path)
            continue

        # Compare file contents
        try:
            with open(artifact_path, "rb") as f:
                artifact_content = f.read()
            with open(working_path, "rb") as f:
                working_content = f.read()

            if artifact_content != working_content:
                print(f"📝 {file_path}: Modified")
                modified_files.append(file_path)

                # Show unified diff for text files
                try:
                    artifact_text = artifact_content.decode("utf-8")
                    working_text = working_content.decode("utf-8")

                    import difflib

                    diff = difflib.unified_diff(
                        artifact_text.splitlines(keepends=True),
                        working_text.splitlines(keepends=True),
                        fromfile=f"artifact/{file_path}",
                        tofile=f"working/{file_path}",
                        lineterm="",
                    )
                    diff_lines = list(diff)
                    if diff_lines:
                        print("".join(diff_lines[:100]))  # Limit to first 100 lines
                        if len(diff_lines) > 100:
                            print(f"... ({len(diff_lines) - 100} more lines)")
                        print()
                except (UnicodeDecodeError, AttributeError):
                    # Binary file, just note it's different
                    print(f"   (Binary file changed)\n")
            else:
                unchanged_files.append(file_path)

        except Exception as e:
            print(f"❌ {file_path}: Error comparing - {e}")

    # Find new files (in working directory but not tracked in current unit or dependencies)
    new_files = working_files - unit_files - dependency_files
    for file_path in sorted(new_files):
        created_files.append(file_path)

    # Print summary
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)

    if created_files:
        print(f"\n➕ Created ({len(created_files)} file(s)):")
        for f in created_files:
            print(f"   - {f}")

    if modified_files:
        print(f"\n📝 Modified ({len(modified_files)} file(s)):")
        for f in modified_files:
            print(f"   - {f}")

    if deleted_files:
        print(f"\n🗑️  Deleted ({len(deleted_files)} file(s)):")
        for f in deleted_files:
            print(f"   - {f}")

    if unchanged_files:
        print(f"\n✅ Unchanged ({len(unchanged_files)} file(s))")

    # Overall status
    has_differences = bool(created_files or modified_files or deleted_files)

    if not has_differences:
        print("\n✅ No differences found. Working directory matches artifact registry.")
    else:
        print("\n⚠️  Differences found between working directory and artifact registry.")
        if created_files or modified_files:
            print("    Use 'laforge add <files>' to update the artifact registry.")

    return True


def cmd_finalize(args: List[str]):
    """laforge finalize [unit_id] - Mark a unit as finalized"""
    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Determine which unit to finalize
    if len(args) >= 1:
        unit_id = args[0]
    else:
        # Use current unit
        unit_info = LaforgeUnit().load()
        if not unit_info:
            print(
                "Error: No current unit. Run 'laforge init' first or provide a unit_id.",
                file=sys.stderr,
            )
            return False
        unit_id = unit_info["unit_id"]

    # Verify unit exists
    unit = db.get_unit(unit_id)
    if not unit:
        print(f"Error: Unit '{unit_id}' not found", file=sys.stderr)
        return False

    # Check if already finalized
    if unit["finalized"]:
        print(f"Unit '{unit_id}' is already finalized", file=sys.stderr)
        return False

    # Mark as finalized
    conn = db.get_connection()
    cursor = conn.cursor()
    try:
        cursor.execute("UPDATE units SET finalized = 1 WHERE unit_id = ?", (unit_id,))
        conn.commit()
        print(f"Finalized unit: '{unit_id}'")
        return True
    except Exception as e:
        print(f"Error finalizing unit: {e}", file=sys.stderr)
        return False
    finally:
        conn.close()


def cmd_next(args: List[str]):
    """laforge next - List units that are ready to work on (not finalized, all dependencies finalized)"""
    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Get all non-finalized units
    conn = db.get_connection()
    cursor = conn.cursor()
    cursor.execute(
        """
        SELECT unit_id, description, created_at
        FROM units
        WHERE finalized = 0
        ORDER BY created_at
        """
    )
    rows = cursor.fetchall()
    conn.close()

    ready_units = []

    for row in rows:
        unit_id = row[0]
        description = row[1]
        created_at = row[2]

        # Check if all dependencies are finalized
        dependencies = db.get_unit_dependencies(unit_id)
        all_deps_finalized = True

        for dep in dependencies:
            dep_unit = db.get_unit(dep["unit_id"])
            if dep_unit and not dep_unit["finalized"]:
                all_deps_finalized = False
                break

        if all_deps_finalized:
            ready_units.append(
                {
                    "unit_id": unit_id,
                    "description": description,
                    "created_at": created_at,
                }
            )

    if not ready_units:
        print("No units are ready to work on.")
        return True

    print(f"Found {len(ready_units)} unit(s) ready to work on:\n")
    for unit in ready_units:
        print(f"  {unit['unit_id']}: {unit['description']}")

    return True


def cmd_update(args: List[str]):
    """laforge update <old_unit_id> <new_unit_id> [output_file] - Generate YAML to update dependencies from old to new unit ID"""
    if len(args) < 2:
        print(
            "Usage: laforge update <old_unit_id> <new_unit_id> [output_file]",
            file=sys.stderr,
        )
        return False

    old_unit_id = args[0]
    new_unit_id = args[1]
    output_file = args[2] if len(args) > 2 else "updates.yaml"

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Verify old unit exists
    old_unit = db.get_unit(old_unit_id)
    if not old_unit:
        print(f"Error: Old unit '{old_unit_id}' not found", file=sys.stderr)
        return False

    # Verify new unit exists
    new_unit = db.get_unit(new_unit_id)
    if not new_unit:
        print(f"Error: New unit '{new_unit_id}' not found", file=sys.stderr)
        return False

    print(f"Planning update from '{old_unit_id}' to '{new_unit_id}'...")
    print()

    # Track all versions that will be created (old_id -> new_id)
    version_mapping = {old_unit_id: new_unit_id}

    # First pass: Find all units that need updating and determine their new IDs
    def collect_affected_units(changed_unit_id: str, visited=None):
        """Recursively find all units affected by a change"""
        if visited is None:
            visited = set()

        if changed_unit_id in visited:
            return
        visited.add(changed_unit_id)

        # Find units that depend on this changed unit
        dependents = db.get_dependents(changed_unit_id)

        for dependent_id in dependents:
            # Skip the new_unit_id itself - it shouldn't be updated
            if dependent_id == new_unit_id:
                print(f"  '{dependent_id}' is the target unit, skipping")
                continue

            # Skip if already mapped
            if dependent_id in version_mapping:
                continue

            dependent = db.get_unit(dependent_id)
            if not dependent:
                continue

            if dependent["finalized"]:
                # Finalized units get a new version
                new_id = db.find_available_version(dependent_id)
                version_mapping[dependent_id] = new_id
                print(f"  '{dependent_id}' (finalized) -> will create '{new_id}'")

                # Recursively check dependents of this unit
                collect_affected_units(dependent_id, visited)
            else:
                # Non-finalized units stay the same (updated in place)
                version_mapping[dependent_id] = dependent_id
                print(f"  '{dependent_id}' (not finalized) -> will update in place")

    print("Analyzing dependency tree...")
    collect_affected_units(old_unit_id)
    print()

    # Second pass: Create operations based on the complete version mapping
    operations = []
    processed = set([old_unit_id, new_unit_id])  # Don't process the initial change

    for old_id, new_id in version_mapping.items():
        if old_id in processed:
            continue
        processed.add(old_id)

        unit = db.get_unit(old_id)
        if not unit:
            continue

        # Get current dependencies
        dependencies = db.get_unit_dependencies(old_id)
        dep_ids = [dep["unit_id"] for dep in dependencies]

        # Map dependencies to their new versions
        updated_deps = [version_mapping.get(dep_id, dep_id) for dep_id in dep_ids]

        # Check if any dependencies actually changed
        deps_changed = updated_deps != dep_ids

        if not deps_changed:
            continue

        if unit["finalized"]:
            # Create COPY_UNIT operation
            operations.append(
                {
                    "action": "COPY_UNIT",
                    "source_id": old_id,
                    "new_id": new_id,
                    "dependencies": updated_deps,
                }
            )
            print(f"Creating COPY_UNIT: {old_id} -> {new_id}")
        else:
            # Create REMOVE and ADD operations for changed dependencies
            deps_to_remove = [
                dep_ids[i] for i in range(len(dep_ids)) if dep_ids[i] != updated_deps[i]
            ]
            deps_to_add = [
                updated_deps[i]
                for i in range(len(updated_deps))
                if dep_ids[i] != updated_deps[i]
            ]

            if deps_to_remove:
                operations.append(
                    {
                        "action": "REMOVE_DEPENDENCIES",
                        "id": old_id,
                        "dependencies": deps_to_remove,
                    }
                )
                print(f"Removing dependencies from {old_id}: {deps_to_remove}")

            if deps_to_add:
                operations.append(
                    {
                        "action": "ADD_DEPENDENCIES",
                        "id": old_id,
                        "dependencies": deps_to_add,
                    }
                )
                print(f"Adding dependencies to {old_id}: {deps_to_add}")

    print()

    # Print summary
    new_versions = {
        k: v for k, v in version_mapping.items() if k != old_unit_id and k != v
    }  # Exclude initial change and in-place updates

    if new_versions:
        print(f"Summary: Will create {len(new_versions)} new version(s):")
        for old_id, new_id in new_versions.items():
            print(f"  {old_id} -> {new_id}")
        print()
    else:
        print(
            "No finalized units found (only in-place updates to non-finalized units)."
        )
        print()

    # Write operations to YAML file
    if not operations:
        print("No operations needed - no units depend on the changed unit.")
        return True

    try:
        with open(output_file, "w") as f:
            yaml.dump(operations, f, default_flow_style=False, sort_keys=False)
        print(f"Wrote {len(operations)} operation(s) to '{output_file}'")
        print(f"Review the changes and apply with: laforge apply {output_file}")
    except Exception as e:
        print(f"Error writing YAML file: {e}", file=sys.stderr)
        return False

    return True


def cmd_apply(args: List[str]):
    """laforge apply <yaml_file> [--dry-run] - Apply operations from a YAML file"""
    if len(args) < 1:
        print("Usage: laforge apply <yaml_file> [--dry-run]", file=sys.stderr)
        return False

    # Parse arguments
    dry_run = "--dry-run" in args
    yaml_file_args = [arg for arg in args if arg != "--dry-run"]

    if len(yaml_file_args) < 1:
        print("Usage: laforge apply <yaml_file> [--dry-run]", file=sys.stderr)
        return False

    yaml_file = Path(yaml_file_args[0])

    if not yaml_file.exists():
        print(f"Error: File '{yaml_file}' not found", file=sys.stderr)
        return False

    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))
    registry = LaforgeRegistry(registry_path)

    if not registry.ensure_exists():
        return False

    # Read and parse YAML file
    try:
        with open(yaml_file, "r") as f:
            operations = yaml.safe_load(f)
    except yaml.YAMLError as e:
        print(f"Error parsing YAML file: {e}", file=sys.stderr)
        return False
    except Exception as e:
        print(f"Error reading file: {e}", file=sys.stderr)
        return False

    # Validate operations BEFORE applying any
    if dry_run:
        print(f"[DRY RUN] Validating operations from '{yaml_file}'...")
    else:
        print(f"Validating operations from '{yaml_file}'...")

    valid, error_msg, id_remapping = validate_yaml_operations(operations, db)
    if not valid:
        print(f"Validation failed: {error_msg}", file=sys.stderr)
        return False

    print(f"Validation successful. Found {len(operations)} operation(s).")
    print()

    # Print detailed summary of operations
    for i, op in enumerate(operations):
        action = op.get("action")
        unit_id = op.get("id")

        if action == "CREATE":
            title = op.get("title", "")
            dependencies = op.get("dependencies", [])

            # Check if this unit was remapped
            original_id = None
            for orig, new in id_remapping.items():
                if new == unit_id:
                    original_id = orig
                    break

            if original_id:
                print(f"{i + 1}. CREATE '{unit_id}' (remapped from '{original_id}')")
            else:
                print(f"{i + 1}. CREATE '{unit_id}'")

            print(f"   Title: {title}")
            if dependencies:
                print(f"   Dependencies: {', '.join(dependencies)}")

        elif action == "COPY_UNIT":
            source_id = op.get("source_id", "")
            new_id = op.get("new_id", "")
            dependencies = op.get("dependencies", [])

            # Check if the new_id was remapped
            original_new_id = None
            for orig, new in id_remapping.items():
                if new == new_id:
                    original_new_id = orig
                    break

            if original_new_id:
                print(
                    f"{i + 1}. COPY_UNIT '{source_id}' -> '{new_id}' (remapped from '{original_new_id}')"
                )
            else:
                print(f"{i + 1}. COPY_UNIT '{source_id}' -> '{new_id}'")

            if dependencies:
                print(f"   Dependencies: {', '.join(dependencies)}")

        elif action == "ADD_DEPENDENCIES":
            dependencies = op.get("dependencies", [])
            print(f"{i + 1}. ADD_DEPENDENCIES to '{unit_id}'")
            print(f"   Adding: {', '.join(dependencies)}")

        elif action == "REMOVE_DEPENDENCIES":
            dependencies = op.get("dependencies", [])
            print(f"{i + 1}. REMOVE_DEPENDENCIES from '{unit_id}'")
            print(f"   Removing: {', '.join(dependencies)}")

        print()

    if dry_run:
        print("[DRY RUN] No changes were made to the database.")
        return True

    print("Applying operations...")

    # Execute operations
    if not execute_yaml_operations(operations, db, registry):
        print("Error: Failed to execute operations", file=sys.stderr)
        return False

    print(f"Successfully applied all operations from '{yaml_file}'")
    return True


def main():
    """Main entry point"""
    if len(sys.argv) < 2:
        print("Usage: laforge <command> [args...]", file=sys.stderr)
        print(
            "Commands: init, create, add, add-dep, rm-dep, checkout, tree, diff, finalize, apply, update, next",
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
        "diff": cmd_diff,
        "finalize": cmd_finalize,
        "apply": cmd_apply,
        "update": cmd_update,
        "next": cmd_next,
    }

    if command not in commands:
        print(f"Unknown command: {command}", file=sys.stderr)
        sys.exit(1)

    success = commands[command](args)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
