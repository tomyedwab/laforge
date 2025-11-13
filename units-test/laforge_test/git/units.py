import base64
import datetime
import pathlib
from dataclasses import dataclass

from .exec import git_exec
from .tree import GitTree

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

        for finalized_unit in finalized_deps.values():
            if finalized_unit:
                self._add_unit_dependency(root_tree, finalized_unit)

        root_tree_hash = root_tree.finalize()

        commit_message = f"Created new unit {unit_id}"
        commit_hash = git_exec("commit-tree", [root_tree_hash], commit_message).strip()

        git_exec("update-ref", [f"refs/heads/{branch_name}", commit_hash])

        return branch_name

    def _add_unit_dependency(self, tree: GitTree, unit: FinalizedUnit):
        deps_tree = tree.get_or_create_path(".lf-deps")
        unit_tree = GitTree.from_hash(unit.tree_hash)
        deps_tree.add_tree(unit.unit_id, unit_tree)

        # Create symlinks!
        def callback(path, item_type, value):
            parsed_path = pathlib.Path(path)
            if item_type == "tree":
                if parsed_path.name.startswith("."):
                    return False
                return True

            if item_type == "blob":
                if path.lower() in INTERNAL_FILES:
                    return False
                parent_dir = parsed_path.parent.as_posix()
                parent_tree = (
                    tree if parent_dir == "." else tree.get_or_create_path(parent_dir)
                )
                target = (
                    len(parsed_path.parts) - 1
                ) * "../" + f".lf-deps/{unit.unit_id}/{path}"
                parent_tree.add_symlink(parsed_path.name, target)

            return True

        unit_tree.traverse(callback)
