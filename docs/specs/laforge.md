## Main agent harness

The `laforge` command is meant to be run on the host machine and provides the
main loop and API server for the agent container. It manages projects and passes
tasks to the agent containers it spawns, capturing logs, validating output and
passing review requests to the user via the API. See README.md for more
information.

The command line arguments for `laforge` are as follows:
```
laforge [options] [command]

Options:
  -h, --help      Show help information
  -v, --version   Show version information

Commands:
  init            Initialize a new project in ~/.laforge/projects/<id>
  step            Run a single step for the given project ID and exit
  run             Run all projects and API server
```
