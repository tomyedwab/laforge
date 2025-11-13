# Main file for laforge utility
"""
This utility is meant to be called by a coding agent to interact with Units
stored in a Git repository and manipulate the current working directory.

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

Units in progress are stored in branches in Git named
"uip/<unit_id>_<timestamp>". Finalized Units are stored as branches named
"u/<unit id>" and consist of a series of commits, each pointing to the tip of a
uip branch.

Since Units depend on other Units, each Unit commit stores the other (finalized)
unit dependencies as parent commits, and those Unit's Git trees are imported
into a ".lf-deps" directory. The files are symlinked into their correct
locations at checkout time using git hooks, but should not be writeable. Each
unit also contains a ".lf-info" directory for storing work-in-progress files
such as PLAN.md, WANTS.md, and REVIEW.md. These are not symlinked into dependent
Units.

The following commands are supported:

laforge create <unit_name> <description> - Creates a new Unit with a unique ID.
laforge add-dep <unit_id> - Adds a dependency to the current Unit with the given unit ID.
laforge rm-dep <unit_id> - Removes a dependency from the current Unit with the given unit ID.
laforge tree - Prints the current Unit and all its dependencies in a tree format.
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
from typing import Dict, List

import yaml

from .git.exec import git_exec
from .git.units import UnitDB

INTERNAL_FILES = [
    "plan.md",
    "review.md",
    "wants.md",
]


class LaforgeDB:
    pass


class LaforgeRegistry:
    pass


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

    branch_name = UnitDB().create_unit(
        unit_id, description, acceptance_criteria, dependencies
    )
    print(f"Successfully created unit {unit_id} in branch {branch_name}")


def cmd_add_dep(args: List[str]):
    """laforge add-dep <unit_id> [<branch_name>]"""
    if len(args) < 1:
        print(
            "Usage: laforge add-dep <unit_id> [<branch_name>]",
            file=sys.stderr,
        )
        return False

    depends_on_unit_id = args[0]
    if len(args) > 1:
        branch_name = args[1]
    else:
        branch_name = git_exec("branch", ["--show-current"]).strip()

    UnitDB().add_dependency(branch_name, depends_on_unit_id)
    print(f"Successfully added dependency {depends_on_unit_id} to branch {branch_name}")


def cmd_rm_dep(args: List[str]):
    """laforge rm-dep <unit_id>"""
    if len(args) < 1:
        print("Usage: laforge rm-dep <unit_id>", file=sys.stderr)
        return False

    depends_on_unit_id = args[0]

    raise NotImplementedError("create not yet implemented")
    """
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
    """


def cmd_tree(args: List[str]):
    """laforge tree - Print Units in tree format starting from current unit"""
    # Get current unit
    unit_info = LaforgeUnit().load()
    if not unit_info:
        print("Error: No current unit. Run 'laforge init' first.", file=sys.stderr)
        return False

    current_unit_id = unit_info["unit_id"]

    raise NotImplementedError("not yet implemented")
    """
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
    """


def cmd_finalize(args: List[str]):
    """laforge finalize [unit_id] - Mark a unit as finalized"""
    raise NotImplementedError("not yet implemented")
    """
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
    """


def cmd_next(args: List[str]):
    """laforge next - List units that are ready to work on (not finalized, all dependencies finalized)"""
    raise NotImplementedError("not yet implemented")
    """
    # Get registry
    registry_path = LaforgeConfig().get_registry_path()
    if not registry_path:
        print("Error: artifact_registry not configured", file=sys.stderr)
        return False

    db = LaforgeDB(str(registry_path / "laforge.db"))

    # Get all non-finalized units

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
    """


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

    raise NotImplementedError("create not yet implemented")
    """
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
    """


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

    raise NotImplementedError("not yet implemented")
    """
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
    """


def main():
    """Main entry point"""
    if len(sys.argv) < 2:
        print("Usage: laforge <command> [args...]", file=sys.stderr)
        print(
            "Commands: create, add-dep, rm-dep, tree, finalize, apply, update, next",
            file=sys.stderr,
        )
        sys.exit(1)

    command = sys.argv[1]
    args = sys.argv[2:]

    commands = {
        "create": cmd_create,
        "add-dep": cmd_add_dep,
        "rm-dep": cmd_rm_dep,
        "tree": cmd_tree,
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
