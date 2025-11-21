import base64
import datetime
import os
import pathlib
import sys
from dataclasses import dataclass

from .exec import git_exec
from .tree import GitTree

INTERNAL_FILES = [
    "plan.md",
    "status.yaml",
    "changelog.md",
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


@dataclass
class FinalizedUnit(object):
    unit_id: str
    commit_hash: str
    tree_hash: str


class UnitDB(object):
    def __init__(self):
        pass

    def get_finalized_unit(self, unit_tag: str) -> FinalizedUnit | None:
        try:
            commit_hash = git_exec(
                "show-ref", [f"refs/tags/u/{unit_tag}", "-s"]
            ).strip()
        except Exception:
            return None
        if commit_hash == "":
            return None
        tree_hash = git_exec("show", [commit_hash, "--format=%T", "--no-patch"]).strip()
        return FinalizedUnit(
            unit_id=unit_tag.split("#", 1)[0],
            commit_hash=commit_hash,
            tree_hash=tree_hash,
        )

    def create_unit(
        self,
        unit_id: str,
        description: str,
        acceptance_criteria: str,
        dependencies: list[str],
    ):
        # First check if we have all the dependencies finalized
        finalized_deps = {
            unit_tag: self.get_finalized_unit(unit_tag) for unit_tag in dependencies
        }
        for unit_tag, finalized in finalized_deps.items():
            if not finalized:
                raise Exception(f"No tag for finalized unit u/{unit_tag} found")

        # TODO: Support creating a unit from one or more existing units
        # TODO: Sometimes the base64 id is not a valid branch name!
        ts = int(datetime.datetime.now(datetime.UTC).timestamp())
        ts_b64 = base64.b64encode(ts.to_bytes(6)).decode()
        branch_name = f"uip/{unit_id}_{ts_b64}"

        root_tree = GitTree()

        # Create a PLAN.md file for the new unit
        root_tree.add_string_blob(
            "PLAN.md",
            PLAN_TEMPLATE.format(
                unit_id=unit_id,
                description=description,
                acceptance_criteria=acceptance_criteria,
                unit_dependencies=dependencies,  # TODO: Format as a tree
            ),
        )
        root_tree_hash = root_tree.finalize()

        commit_message = f"Created new unit {unit_id}"
        commit_hash = git_exec("commit-tree", [root_tree_hash], commit_message).strip()

        for finalized_unit in finalized_deps.values():
            if finalized_unit:
                self._add_unit_dependency(root_tree, finalized_unit)
                merged_hash = root_tree.finalize()
                commit_message = f"Added unit dependency {finalized_unit.unit_id}"
                commit_hash = git_exec(
                    "commit-tree",
                    [merged_hash, "-p", commit_hash, "-p", finalized_unit.commit_hash],
                    commit_message,
                ).strip()

        git_exec("update-ref", [f"refs/heads/{branch_name}", commit_hash])

        return branch_name

    def import_unit(
        self,
        unit_id: str,
        description: str,
        files: list[str],
    ):
        unit_tag = f"u/{unit_id}"
        if self.get_finalized_unit(unit_tag):
            raise Exception(f"Finalized unit {unit_tag} already exists!")

        root_tree = GitTree()

        # Import files from working directory
        for file_path in files:
            path = pathlib.Path(file_path)
            if not path.exists():
                raise Exception(f"File '{file_path}' does not exist")
            if not path.is_file():
                raise Exception(f"'{file_path}' is not a file")

            # Hash the file and add as blob
            blob_hash = git_exec("hash-object", ["-w", str(path)]).strip()

            # Handle nested paths
            if "/" in file_path:
                parent_path = str(path.parent)
                parent_tree = root_tree.get_or_create_path(parent_path)
                parent_tree.add_hashed_blob(path.name, blob_hash)
            else:
                root_tree.add_hashed_blob(file_path, blob_hash)

        # Create a PLAN.md file for the new unit
        root_tree.add_string_blob(
            "PLAN.md",
            PLAN_TEMPLATE.format(
                unit_id=unit_id,
                description=description,
                acceptance_criteria="(Imported from existing files)",
                unit_dependencies="",
            ),
        )
        root_tree_hash = root_tree.finalize()

        commit_message = f"Imported unit {unit_id} with {len(files)} file(s)"
        commit_hash = git_exec("commit-tree", [root_tree_hash], commit_message).strip()

        git_exec("tag", [unit_tag, commit_hash])

        return unit_tag

    def add_dependency(self, unit_branch: str, dependency_unit_tag: str):
        if not unit_branch.startswith("uip/"):
            raise Exception("Dependencies can only be added to units in progress")
        try:
            commit_hash = git_exec(
                "show-ref", [f"refs/heads/{unit_branch}", "-s"]
            ).strip()
            root_tree_hash = git_exec(
                "show", [commit_hash, "--format=%T", "--no-patch"]
            ).strip()
        except Exception:
            raise Exception(f"Unit in-progress branch {unit_branch} not found.")

        finalized_unit = self.get_finalized_unit(dependency_unit_tag)
        if not finalized_unit:
            raise Exception(f"Finalized unit {dependency_unit_tag} not found.")

        root_tree = GitTree.from_hash(root_tree_hash)
        self._add_unit_dependency(root_tree, finalized_unit)
        merged_hash = root_tree.finalize()
        commit_message = f"Added unit dependency {finalized_unit.unit_id}"
        commit_hash = git_exec(
            "commit-tree",
            [merged_hash, "-p", commit_hash, "-p", finalized_unit.commit_hash],
            commit_message,
        ).strip()

        git_exec("update-ref", [f"refs/heads/{unit_branch}", commit_hash])

    def delete_dependency(self, unit_branch: str, dependency_unit_id: str):
        if not unit_branch.startswith("uip/"):
            raise Exception("Dependencies can only be removed from units in progress")
        try:
            commit_hash = git_exec(
                "show-ref", [f"refs/heads/{unit_branch}", "-s"]
            ).strip()
            root_tree_hash = git_exec(
                "show", [commit_hash, "--format=%T", "--no-patch"]
            ).strip()
        except Exception:
            raise Exception(f"Unit in-progress branch {unit_branch} not found.")

        root_tree = GitTree.from_hash(root_tree_hash)
        self._remove_unit_dependency(root_tree, dependency_unit_id)
        merged_hash = root_tree.finalize()
        commit_message = f"Removed unit dependency {dependency_unit_id}"
        commit_hash = git_exec(
            "commit-tree",
            [merged_hash, "-p", commit_hash],
            commit_message,
        ).strip()

        git_exec("update-ref", [f"refs/heads/{unit_branch}", commit_hash])

    def merge_unit(self, unit_ref):
        # Handle both finalized units (u/<tag>) and units in progress (uip/<branch>)
        if unit_ref.startswith("u/"):
            # Finalized unit - use tag
            unit_tag = unit_ref[2:]  # Remove "u/" prefix
            finalized_unit = self.get_finalized_unit(unit_tag)
            if not finalized_unit:
                print(f"Error: Finalized unit '{unit_ref}' not found", file=sys.stderr)
                return False
            unit_tree = GitTree.from_hash(finalized_unit.tree_hash)
        elif unit_ref.startswith("uip/"):
            # Unit in progress - use branch
            try:
                commit_hash = git_exec(
                    "show-ref", [f"refs/heads/{unit_ref}", "-s"]
                ).strip()
                if not commit_hash:
                    print(
                        f"Error: Unit in progress branch '{unit_ref}' not found",
                        file=sys.stderr,
                    )
                    return False
                tree_hash = git_exec(
                    "show", [commit_hash, "--format=%T", "--no-patch"]
                ).strip()
                unit_tree = GitTree.from_hash(tree_hash)
            except Exception as e:
                print(
                    f"Error: Unit in progress branch '{unit_ref}' not found",
                    file=sys.stderr,
                )
                return False
        else:
            print(
                f"Error: Unit reference must start with 'u/' or 'uip/'", file=sys.stderr
            )
            return False

        # Collect all files to copy (excluding internal files and .lf-deps)
        files_to_copy = []  # List of (relative_path, blob_hash)

        def collect_files(tree, path, item_type, value):
            parsed_path = pathlib.Path(path)

            # Skip .lf-deps directory
            if item_type == "tree" and parsed_path.name == ".lf-deps":
                return False

            # Skip internal files
            if item_type == "blob" and path.lower() in INTERNAL_FILES:
                return True

            # Collect blob files (resolve symlinks to actual blobs)
            if item_type == "blob":
                files_to_copy.append((path, value))
            elif item_type == "symlink":
                # Resolve symlink to get the actual blob
                target = git_exec("cat-file", ["-p", value]).strip()
                # If it points to .lf-deps, follow it
                if ".lf-deps/" in target:
                    # Construct the full path by resolving the relative symlink
                    symlink_dir = parsed_path.parent
                    target_path = (symlink_dir / target).resolve()
                    # Convert back to relative path from root
                    try:
                        # Extract the path within .lf-deps
                        parts = target.split(".lf-deps/", 1)
                        if len(parts) == 2:
                            # Navigate from root: count ../ to determine depth
                            depth = target.count("../")
                            dep_path = parts[1]

                            # Resolve the blob by navigating the tree
                            dep_parts = dep_path.split("/")
                            current_tree = unit_tree

                            # Navigate to .lf-deps
                            if ".lf-deps" in current_tree.trees:
                                current_tree = current_tree.trees[".lf-deps"]

                                # Navigate through the dependency path
                                for i, part in enumerate(dep_parts[:-1]):
                                    if part in current_tree.trees:
                                        current_tree = current_tree.trees[part]
                                    elif part in current_tree.symlinks:
                                        # Follow symlink
                                        link_target = git_exec(
                                            "cat-file",
                                            ["-p", current_tree.symlinks[part]],
                                        ).strip()
                                        # This gets complex, skip for now
                                        break
                                    else:
                                        break
                                else:
                                    # Get the final blob
                                    filename = dep_parts[-1]
                                    if filename in current_tree.blobs:
                                        blob_hash = current_tree.blobs[filename]
                                        files_to_copy.append((path, blob_hash))
                    except Exception:
                        pass

            return True

        unit_tree.traverse(collect_files)

        # Check which files already exist in working directory
        files_to_overwrite = []
        identical_files = []
        new_files = []

        for file_path, blob_hash in files_to_copy:
            if os.path.exists(file_path):
                # Check if the file content is identical
                # Hash the existing file and compare with blob_hash
                try:
                    existing_hash = git_exec("hash-object", [file_path]).strip()

                    if existing_hash == blob_hash:
                        identical_files.append(file_path)
                    else:
                        files_to_overwrite.append(file_path)
                except Exception:
                    # If we can't hash it, treat it as different
                    files_to_overwrite.append(file_path)
            else:
                new_files.append(file_path)

        # Print summary
        print(f"Merging unit '{unit_ref}' into working directory:")
        print(f"  New files: {len(new_files)}")
        print(f"  Identical files (will skip): {len(identical_files)}")
        print(f"  Files to overwrite: {len(files_to_overwrite)}")
        print()

        if new_files:
            print("New files to create:")
            for file_path in sorted(new_files):
                print(f"  + {file_path}")
            print()

        if identical_files:
            print("Identical files (skipping):")
            for file_path in sorted(identical_files):
                print(f"  = {file_path}")
            print()

        if files_to_overwrite:
            print("Files that will be overwritten:")
            for file_path in sorted(files_to_overwrite):
                print(f"  ! {file_path}")
            print()

            # Prompt for confirmation
            response = input("Overwrite existing files? [y/N]: ").strip().lower()
            if response not in ["y", "yes"]:
                print("Merge cancelled.")
                return False

        # Copy all files (except identical ones)
        print("Copying files...")
        files_written = 0
        for file_path, blob_hash in files_to_copy:
            # Skip identical files
            if file_path in identical_files:
                continue

            # Create parent directories if needed
            parent_dir = os.path.dirname(file_path)
            if parent_dir:
                os.makedirs(parent_dir, exist_ok=True)

            # Get blob content as bytes directly using subprocess
            blob_content = git_exec("cat-file", ["-p", blob_hash])
            with open(file_path, "w") as f:
                f.write(blob_content)

            files_written += 1

        return files_written

    def _update_transitive_dependencies(self, deps_tree: GitTree) -> dict[str, GitTree]:
        transitive_deps = {}
        dep_stack = [(name, tree) for name, tree in deps_tree.trees.items()]
        while len(dep_stack) > 0:
            dep_path, dep_tree = dep_stack.pop()
            sub_deps_tree = dep_tree.trees.get(".lf-deps", None)
            if sub_deps_tree:
                for sub_dep_name, sub_dep_tree in sub_deps_tree.trees.items():
                    sub_dep_path = f"{dep_path}/.lf-deps/{sub_dep_name}"
                    if sub_dep_name not in transitive_deps:
                        transitive_deps[sub_dep_name] = (sub_dep_path, sub_dep_tree)
                    dep_stack.append((sub_dep_path, sub_dep_tree))

        deps_tree.symlinks = {}
        for dep_name, (target, _) in transitive_deps.items():
            if dep_name not in deps_tree.trees:
                deps_tree.add_symlink(dep_name, target)

        return {dep_name: tree for dep_name, (_, tree) in transitive_deps.items()}

    def _add_unit_dependency(self, tree: GitTree, unit: FinalizedUnit):
        deps_tree = tree.get_or_create_path(".lf-deps")
        unit_tree = GitTree.from_hash(unit.tree_hash)
        deps_tree.add_tree(unit.unit_id, unit_tree)

        # Create symlinks in .lf-deps to transitive dependencies so each
        # dependency is located at exactly one path
        transitive_trees = self._update_transitive_dependencies(deps_tree)

        # Create blob symlinks into the first-level .lf-deps paths
        def callback(dep_id, path, item_type):
            parsed_path = pathlib.Path(path)
            if item_type == "tree":
                return parsed_path.name != ".lf-deps"

            if item_type == "blob":
                if path.lower() in INTERNAL_FILES:
                    return False
                parent_dir = parsed_path.parent.as_posix()
                parent_tree = (
                    tree if parent_dir == "." else tree.get_or_create_path(parent_dir)
                )
                target = (
                    len(parsed_path.parts) - 1
                ) * "../" + f".lf-deps/{dep_id}/{path}"
                parent_tree.add_symlink(parsed_path.name, target)

            return True

        unit_tree.traverse(
            lambda tree, path, item_type, _: callback(unit.unit_id, path, item_type)
        )
        for transitive_dep_name, transitive_dep_tree in transitive_trees.items():
            transitive_dep_tree.traverse(
                lambda tree, path, item_type, _: callback(
                    transitive_dep_name, path, item_type
                )
            )

    def _remove_unit_dependency(self, tree: GitTree, unit_id: str):
        if not tree.trees[".lf-deps"]:
            raise Exception(f"Branch has no dependency {unit_id}")
        deps_tree = tree.trees[".lf-deps"]
        if unit_id not in deps_tree.trees:
            raise Exception(f"Branch has no dependency {unit_id}")
        del deps_tree.trees[unit_id]
        # Clean out transitive dependency symlinks
        _ = self._update_transitive_dependencies(deps_tree)
        remaining_deps = set(deps_tree.trees.keys()) | set(deps_tree.symlinks.keys())
        symlinks_to_remove = []

        def callback(tree, path, item_type, value):
            parsed_path = pathlib.Path(path)
            # Don't recuse into .lf-deps
            if item_type == "tree":
                return parsed_path.name != ".lf-deps"

            if item_type == "symlink":
                target = git_exec("cat-file", ["-p", value]).strip()
                try:
                    idx = target.index(".lf-deps")
                    unit_id = target[idx + 9 :].split("/", 1)[0]
                    if unit_id not in remaining_deps:
                        symlinks_to_remove.append((tree, parsed_path.name))
                except ValueError:
                    pass

        tree.traverse(callback)

        for tree, name in symlinks_to_remove:
            del tree.symlinks[name]
