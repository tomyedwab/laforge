import base64
import datetime
import pathlib
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
        # TODO: Sometimes the base64 id is not a valid branch name!
        ts = int(datetime.datetime.now(datetime.UTC).timestamp())
        ts_b64 = base64.b64encode(ts.to_bytes(6)).decode()
        branch_name = f"uip/{unit_id}_{ts_b64}"

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

        git_exec("update-ref", [f"refs/heads/{branch_name}", commit_hash])

        return branch_name

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
