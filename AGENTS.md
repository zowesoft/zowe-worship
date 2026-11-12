# Agent notes for Zopresent

## Always rebuild after a change

Every change ends with a full Wails rebuild of `bin/zopresent.exe`. A green
`go build`, `tsc` or test run is not enough: the app the team runs is the
packaged binary, and the frontend is baked into it, so an un-rebuilt tree means
the change is not actually testable in the app.

Run the checks, rebuild, relaunch, and say so in the summary. If the rebuild
cannot run, report that instead of claiming the change is live.

### When the change touches Go

1. `gofmt -w <changed files>`
2. `CGO_ENABLED=0 go build ./...`
3. `CGO_ENABLED=0 go test ./...` (add `-count=1` when re-running after a fix, or
   the suite replays a cached pass)
4. Regenerate bindings if an exported app method, model or event changed:
   `/c/Users/JohnCode/go/bin/wails3 generate bindings -f '-tags production' -clean=true -ts -i`
   (~15 s; rewrites `frontend/bindings/**`)

### When the change touches the frontend

1. `cd frontend && npm run typecheck && npm run build`
2. Still rebuild the binary — Vite output is embedded in the exe.

### Rebuild and relaunch

`wails3 build` runs `go mod tidy`, icons, bindings, the Vite production build and
then the production `go build`. It takes several minutes and outlasts a 5-minute
shell timeout, so start it in the background with output redirected to a log,
then poll that log and `bin/zopresent.exe`.

```bash
/c/Users/JohnCode/go/bin/wails3 build > /tmp/zopresent-build.log 2>&1   # background
ls -l bin/zopresent.exe                                        # timestamp must advance
cmd //c start "" "bin\\zopresent.exe"
powershell -NoProfile -Command "Get-Process zopresent | Select-Object Id,MainWindowTitle"
```

The rebuilt binary is only verified when the exe timestamp has moved and the
process is running with the `Zopresent` window title.

`scripts/rebuild.sh` does all of that in one command: build, side-effect
cleanup, relaunch, and the window check, exiting non-zero if any step fails.
Use it instead of repeating the steps by hand.

## Commits and staging

### Commit messages

History follows Conventional Commits: `type(scope): imperative summary`, with a
blank line and a body that says *why* the change was made. Types in use are
`feat`, `fix`, `test`, `docs`, `refactor` and `chore`; the scope names the area
(`detection`, `media`, `database`, `bible`, …) and is omitted for repository-wide
docs. Keep the subject under ~72 characters, lowercase after the colon, no
trailing period.

No attribution trailers. Do not add `Co-Authored-By`, `Generated with …` or any
other footer; the history has none and reviewers expect the message to end with
the body.

Write the message to a file and commit with `git commit -F <file>` when it
contains quotes, backticks or em dashes. A `-m "$(cat <<'EOF' …)"` form breaks
on an unbalanced quote inside the body.

### Staging

The working tree is shared and frequently holds someone else's unfinished work.
Before staging anything:

1. `git status --porcelain` and list only the paths this change actually touched.
2. Stage by explicit path. Never use `git add -A`, `git add .` or `git commit -a`.
3. If an owned file also carries someone else's hunks, stage only your hunks:
   filter the patch and apply it with `git apply --cached --recount`, then check
   that no removed lines were staged and that every staged line is yours.
4. Commit one slice at a time, and only once its checks pass.
5. Verify the result with `git show --stat <sha>` and confirm no unrelated file
   (and none of the WIP paths from step 1) appears in it.

Commit promptly after a slice passes its checks. Work left uncommitted has been
swept into other people's commits on this tree, which loses the reason for the
change and muddies the history.

Update `docs/FEATURE_IMPLEMENTATION_TRACKER.md` in the same commit as the slice
it describes, with claims that match what the code now does.

Do not push, tag, rewrite history or reset branches unless asked.

### Build side effects to clean up

- `wails3 build` stops running `zopresent` processes itself; relaunch after.
- The Vite step deletes the tracked `frontend/dist/.gitkeep`. Restore it with
  `git checkout -- frontend/dist/.gitkeep` so it is not staged as a deletion.
- The bindings step can leave a stray untracked `frontend/frontend/` directory.
  Delete it before committing.
- Use `wails3 build` directly: `wails3` is at `/c/Users/JohnCode/go/bin/` and
  `task` is not on PATH.
