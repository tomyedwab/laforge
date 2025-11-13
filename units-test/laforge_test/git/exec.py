import subprocess


def git_exec(command: str, args: list[str], input: str | None = None) -> str:
    """Run the git executable with given command and arguments.

    If `input` is provided, pipe the string into the subprocess's stdin.

    Returns True if the command ran successfully.
    """
    cmd = ["git", command] + args

    result = subprocess.run(
        cmd,
        input=input,
        text=True if input is not None else False,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise Exception(
            f"Subprocess returned code: {result.returncode}\n>> stderr:\n{result.stderr}"
        )
    return result.stdout.decode() if isinstance(result.stdout, bytes) else result.stdout
