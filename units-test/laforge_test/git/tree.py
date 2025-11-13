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

    @classmethod
    def from_hash(cls, tree_hash: str) -> "GitTree":
        """Parse a tree object from its hash and return a GitTree with fixed_hash set."""
        tree_content = git_exec("cat-file", ["-p", tree_hash])
        tree = cls(fixed_hash=tree_hash)

        if not tree_content.strip():
            return tree

        for line in tree_content.strip().split("\n"):
            if not line:
                continue

            # Parse: <mode> <type> <hash>\t<name>
            parts = line.split("\t", 1)
            if len(parts) != 2:
                continue

            meta, name = parts
            meta_parts = meta.split()
            if len(meta_parts) != 3:
                continue

            mode, obj_type, obj_hash = meta_parts

            if mode == "040000" and obj_type == "tree":
                tree.trees[name] = cls.from_hash(obj_hash)
            elif mode == "100644" and obj_type == "blob":
                tree.blobs[name] = obj_hash
            elif mode == "120000" and obj_type == "blob":
                tree.symlinks[name] = obj_hash

        return tree

    def traverse(self, callback, path: str = ""):
        """
        Traverse the tree structure, calling callback for each item.

        Args:
            callback: Function called with (path, item_type, value) where:
                - path: str - the full path to the item
                - item_type: str - "tree", "blob", or "symlink"
                - value: GitTree | str - GitTree object for trees, hash for blobs/symlinks
            path: str - current path prefix (used internally for recursion)
        """
        # Visit all subtrees
        for tree_name, tree in self.trees.items():
            tree_path = f"{path}/{tree_name}" if path else tree_name
            if callback(tree_path, "tree", tree):
                tree.traverse(callback, tree_path)

        # Visit all blobs
        for blob_name, blob_hash in self.blobs.items():
            blob_path = f"{path}/{blob_name}" if path else blob_name
            callback(blob_path, "blob", blob_hash)

        # Visit all symlinks
        for link_name, link_hash in self.symlinks.items():
            link_path = f"{path}/{link_name}" if path else link_name
            callback(link_path, "symlink", link_hash)

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
