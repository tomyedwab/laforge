from dataclasses import dataclass

from .exec import git_exec


@dataclass
class GitTree(object):
    fixed_hash: str | None
    trees: dict[str, "GitTree"]
    blobs: dict[str, str]
    symlinks: dict[str, str]

    def __init__(self, fixed_hash: str | None = None):
        self.fixed_hash = fixed_hash
        self.trees = {}
        self.blobs = {}
        self.symlinks = {}

    def add_tree(self, name: str, tree: "GitTree"):
        self.trees[name] = tree

    def add_hashed_tree(self, name: str, tree_hash: str):
        self.trees[name] = GitTree(fixed_hash=tree_hash)

    def add_hashed_blob(self, name: str, blob_hash: str):
        self.blobs[name] = blob_hash

    def add_string_blob(self, name: str, blob_str: str):
        blob_hash = git_exec("hash-object", ["-w", "--stdin"], blob_str).strip()
        self.blobs[name] = blob_hash

    def add_symlink(self, name: str, target: str):
        target_hash = git_exec("hash-object", ["-w", "--stdin"], target).strip()
        self.symlinks[name] = target_hash

    def get_or_create_path(self, path: str) -> "GitTree":
        path_parts = path.split("/")
        cur_tree = self
        for path_part in path_parts:
            if path_part not in cur_tree.trees:
                cur_tree.trees[path_part] = GitTree()
            cur_tree = cur_tree.trees[path_part]
        return cur_tree

    def finalize(self) -> str:
        if self.fixed_hash:
            return self.fixed_hash

        tree_content = "\n".join(
            [
                f"040000 tree {tree.finalize()}\t{tree_name}"
                for tree_name, tree in self.trees.items()
            ]
            + [
                f"100644 blob {blob_hash}\t{blob_name}"
                for blob_name, blob_hash in self.blobs.items()
            ]
            + [
                f"120000 blob {link_hash}\t{link_name}"
                for link_name, link_hash in self.symlinks.items()
            ]
        )
        return git_exec("mktree", [], tree_content).strip()
