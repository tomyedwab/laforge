# laforge-test

A utility for managing Units and dependencies in a project with artifact registry.

## Installation

Install the package in development mode:

```bash
pip install -e .
```

Or install from a specific directory:

```bash
pip install /path/to/laforge-test
```

## Usage

After installation, you can run `laforge-test` from any directory:

```bash
laforge-test <command> [args...]
```

### Available Commands

- `laforge-test init` - Creates a new root Unit in the project
- `laforge-test create <unit_name> <description>` - Creates a new Unit
- `laforge-test update <files...>` - Updates the current Unit with new artifacts
- `laforge-test add-dep <unit_id> <version> [--want <want_id>]` - Adds a dependency
- `laforge-test rm-dep <unit_id>` - Removes a dependency
- `laforge-test checkout <unit_id> <version>` - Updates to the given version
- `laforge-test find <text> [--search-artifacts]` - Searches for artifacts
- `laforge-test tree` - Prints the current Unit and dependencies in tree format
- `laforge-test wants <string>` - Creates an artifact from a string description
- `laforge-test wants-file <file>` - Adds a file as an artifact and records it as a want

## What is a Unit?

A Unit has a unique identifier, a description, and some number of artifacts (files) that capture the requirements/specification, design decisions, implementation, and test cases for a small functional unit within the project. Units can depend on other Units, forming an acyclic graph of dependencies. Once finalized, that version of the Unit is immutable; however, a new version can be created with reference to the previous version.

## License

MIT
