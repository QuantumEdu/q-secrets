#!/usr/bin/env python3
"""Linux integration checks; only synthetic secrets in isolated temp vaults.
Usage: python3 tests/cli_hardening.py /absolute/path/to/q-secrets [path/to/age]
"""
import json
import os
import pathlib
import pty
import re
import select
import signal
import sqlite3
import subprocess
import sys
import tempfile
import time

if sys.platform != "linux":
    raise SystemExit("This integration suite requires Linux to isolate the D-Bus keychain.")

binary = str(pathlib.Path(sys.argv[1]).resolve())
age_binary = str(pathlib.Path(sys.argv[2]).resolve()) if len(sys.argv) > 2 else None

with tempfile.TemporaryDirectory(prefix="q-secrets-e2e-") as directory:
    temp = pathlib.Path(directory)
    env = dict(os.environ, XDG_CONFIG_HOME=directory, PARENT_ONLY="must-be-removed")
    # Never touch the user's keychain or vault. init creates a throwaway key;
    # the missing D-Bus socket prevents keychain writes on Linux.
    env["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + directory + "/missing-bus"
    env.pop("Q_SECRET_KEY", None)

    def cli(*args, value=None, code=0):
        result = subprocess.run([binary, *args], input=value, capture_output=True, env=env, timeout=15)
        assert result.returncode == code, (args[:2], result.returncode, result.stderr.decode())
        return result

    result = cli("init", value=b"1\n\n")
    key = re.search(rb"AGE-SECRET-KEY-[A-Z0-9]+", result.stdout).group().decode()
    public = re.search(rb"age1[a-z0-9]+", result.stdout).group().decode()
    env["Q_SECRET_KEY"] = key
    secret = b"line1\n'quoted' $PATH `id` \\ end\n"
    cli("add", "p", "TOKEN", "--stdin", value=secret)
    assert cli("get", "p", "TOKEN").stdout == secret
    bad = cli("add", "p", "TOKEN=do-not-echo", code=1)
    assert b"do-not-echo" not in bad.stdout + bad.stderr
    cli("add", "p", "Q_SECRET_KEY", "--stdin", value=b"bad", code=1)
    cli("add", "p", "BAD-NAME", "--stdin", value=b"bad", code=1)
    cli("add", "p", "--allow-argv", "LEGACY=test")
    cli("update", "p", "LEGACY", "--stdin", value=b"updated")
    assert cli("get", "p", "LEGACY").stdout == b"updated"
    listed = json.loads(cli("list", "p", "--output", "json").stdout)
    assert all("value" not in row for row in listed)
    assert secret not in cli("list", "p").stdout
    exported = json.loads(cli("export", "--project", "p").stdout)
    assert next(row["value"] for row in exported if row["key"] == "TOKEN").encode() == secret
    shell = cli("get", "p", "TOKEN", "--output", "env").stdout
    value = subprocess.check_output(["sh", "-c", shell.decode() + '\nprintf "%s" "$TOKEN"'])
    assert value == secret
    backup = temp / "backup.json"
    cli("export", "--file", str(backup))
    assert backup.stat().st_mode & 0o777 == 0o600
    contents = backup.read_bytes()
    cli("export", "--file", str(backup), code=1)
    assert backup.read_bytes() == contents
    link = temp / "symlink.json"
    link.symlink_to(backup)
    cli("export", "--file", str(link), code=1)
    assert backup.read_bytes() == contents
    cli("add", "other", "TOKEN", "--stdin", value=b"other")
    assert cli("export", "--output", "env", code=1).stdout == b""
    cli("import", value=json.dumps(exported).encode())

    child = 'import os,json; print(json.dumps({k:os.getenv(k) for k in ["TOKEN","Q_SECRET_KEY","PARENT_ONLY"]}))'
    inherited = json.loads(cli("run", "p", "--", sys.executable, "-c", child).stdout)
    assert inherited == {"TOKEN": secret.decode(), "Q_SECRET_KEY": None, "PARENT_ONLY": "must-be-removed"}
    for clean in ("--clean-env", "--no-inherit-env"):
        clean_result = json.loads(cli("run", "p", clean, "--", sys.executable, "-c", child).stdout)
        assert clean_result == {"TOKEN": secret.decode(), "Q_SECRET_KEY": None, "PARENT_ONLY": None}
    for flags in ([], ["--watch"]):
        cli("run", "p", *flags, "--", sys.executable, "-c", "raise SystemExit(37)", code=37)
        cli("run", "p", *flags, "--", "sh", "-c", "kill -TERM $$", code=143)

    # A real PTY checks that the default interactive prompt disables echo.
    master, slave = pty.openpty()
    prompt = subprocess.Popen([binary, "add", "p", "PROMPTED"], stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    transcript = b""
    try:
        deadline = time.monotonic() + 5
        while b"Secret value:" not in transcript:
            assert time.monotonic() < deadline, "prompt did not appear"
            if select.select([master], [], [], 0.1)[0]:
                transcript += os.read(master, 4096)
        # ReadPassword disables echo immediately after writing the prompt.
        import termios
        while termios.tcgetattr(master)[3] & termios.ECHO:
            assert time.monotonic() < deadline
            time.sleep(0.01)
        os.write(master, b"hidden-pty-value\n")
        assert prompt.wait(timeout=5) == 0
        while select.select([master], [], [], 0.1)[0]:
            try:
                chunk = os.read(master, 4096)
            except OSError:
                break
            if not chunk:
                break
            transcript += chunk
        assert b"hidden-pty-value" not in transcript
    finally:
        if prompt.poll() is None:
            prompt.kill()
            prompt.wait()
        os.close(master)
    assert cli("get", "p", "PROMPTED").stdout == b"hidden-pty-value"

    # Watch a real WAL-backed DB while another CLI updates secrets.
    log = temp / "watch.jsonl"
    app = ('import json,os,time; '
           'f=open(' + repr(str(log)) + ',"a"); '
           'f.write(json.dumps([os.getpid(),os.getenv("TOKEN"),os.getenv("Q_SECRET_KEY"),os.getenv("PARENT_ONLY")])+"\\n"); '
           'f.close(); time.sleep(60)')
    watcher = subprocess.Popen([binary, "run", "p", "--watch", "--clean-env", "--", sys.executable, "-c", app], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def records():
        return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

    def wait_count(count):
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            rows = records()
            if len(rows) >= count:
                return rows
            assert watcher.poll() is None, "watch exited before restart"
            time.sleep(0.05)
        raise AssertionError("watch restart timed out")

    try:
        rows = wait_count(1)
        assert rows[0][1:] == [secret.decode(), None, None]
        cli("add", "other", "TOKEN", "--stdin", value=b"changed-other-project")
        time.sleep(0.5)
        assert len(records()) == 1, "unrelated project caused restart"
        cli("add", "p", "TOKEN", "--stdin", value=secret)
        time.sleep(0.5)
        assert len(records()) == 1, "identical value caused restart"
        cli("update", "p", "TOKEN", "--stdin", value=b"rotated")
        assert wait_count(2)[1][1:] == ["rotated", None, None]
        watcher.send_signal(signal.SIGTERM)
        watcher.communicate(timeout=8)
        assert watcher.returncode == 143
    finally:
        if watcher.poll() is None:
            watcher.send_signal(signal.SIGTERM)
            watcher.communicate(timeout=8)

    # Independent age CLI compatibility in both directions, using the SQLite
    # ciphertext directly and then inserting a legacy CLI-generated ciphertext.
    if age_binary:
        db_path = temp / "q-secret" / "q-secret.db"
        # SQLite's TEXT is binary in this schema, so read raw bytes explicitly.
        with sqlite3.connect(db_path) as db:
            db.text_factory = bytes
            encrypted = db.execute("SELECT value_enc FROM secrets WHERE project_id='p' AND key_name='TOKEN'").fetchone()[0]
        # age decrypt can read the identity from stdin if ciphertext is a file.
        ciphertext_path = temp / "ciphertext.age"
        ciphertext_path.write_bytes(encrypted)
        decrypted = subprocess.check_output([age_binary, "-d", "-i", "-", str(ciphertext_path)], input=key.encode())
        assert decrypted == b"rotated"
        for flags in ([], ["-a"]):
            legacy = subprocess.check_output([age_binary, *flags, "-r", public], input=b"legacy-age-ciphertext")
            with sqlite3.connect(db_path) as db:
                db.execute("UPDATE secrets SET value_enc=CAST(? AS TEXT) WHERE project_id='p' AND key_name='TOKEN'", (legacy,))
            assert cli("get", "p", "TOKEN").stdout == b"legacy-age-ciphertext"


print("CLI hardening: passed (stdin, hidden prompt, formats, environment, exits/signals, WAL watch" + (", age compatibility" if age_binary else "") + ")")
