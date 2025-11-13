import base64
import datetime
from dataclasses import dataclass

from .exec import git_exec

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
        ts = int(datetime.datetime.now(datetime.UTC).timestamp())
        ts_b64 = base64.b64encode(ts.to_bytes(6)).decode()
        branch_name = f"uip/{unit_id}_{ts_b64}"

        # Create a PLAN.md file for the new unit
        plan_contents = PLAN_TEMPLATE.format(
            unit_id=unit_id,
            description=description,
            acceptance_criteria=acceptance_criteria,
            unit_dependencies=dependencies,  # TODO: Format as a tree
        )
        plan_file_hash = git_exec(
            "hash-object", ["-w", "--stdin"], plan_contents
        ).strip()

        deps_tree_hash = None
        if len(finalized_deps) > 0:
            # TODO: Move these to a merge commit so they can be filtered out of diffs?
            deps_tree = "\n".join(
                [
                    f"040000 tree {dep.tree_hash}\t{dep.unit_id}"
                    for dep in finalized_deps.values()
                ]
            )
            deps_tree_hash = git_exec("mktree", [], deps_tree).strip()

        root_tree = "\n".join(
            [f"100644 blob {plan_file_hash}\tPLAN.md"]
            + (
                [f"040000 tree {deps_tree_hash}\t.lf-deps"]
                if deps_tree_hash is not None
                else []
            )
        )
        root_tree_hash = git_exec("mktree", [], root_tree).strip()

        commit_message = f"Created new unit {unit_id}"
        commit_hash = git_exec("commit-tree", [root_tree_hash], commit_message).strip()

        git_exec("update-ref", [f"refs/heads/{branch_name}", commit_hash])

        return branch_name
