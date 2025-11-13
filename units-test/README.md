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

## What is a Unit?

A Unit has a unique identifier, a description, and some number of artifacts (files) that capture the requirements/specification, design decisions, implementation, and test cases for a small functional unit within the project. Units can depend on other Units, forming an acyclic graph of dependencies. Once finalized, that version of the Unit is immutable; however, a new version can be created with reference to the previous version.
