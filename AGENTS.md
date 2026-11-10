# Agent notes for Zowe Worship

## Always rebuild after a change

Every change ends with a full Wails rebuild of `bin/zowe-worship.exe`. A green
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
then poll that log and `bin/zowe-worship.exe`.

```bash
/c/Users/JohnCode/go/bin/wails3 build > /tmp/zowe-build.log 2>&1   # background
ls -l bin/zowe-worship.exe                                        # timestamp must advance
cmd //c start "" "bin\\zowe-worship.exe"
powershell -NoProfile -Command "Get-Process zowe-worship | Select-Object Id,MainWindowTitle"
```

The rebuilt binary is only verified when the exe timestamp has moved and the
process is running with the `Zowe Worship` window title.

### Build side effects to clean up

- `wails3 build` stops running `zowe-worship` processes itself; relaunch after.
- The Vite step deletes the tracked `frontend/dist/.gitkeep`. Restore it with
  `git checkout -- frontend/dist/.gitkeep` so it is not staged as a deletion.
- The bindings step can leave a stray untracked `frontend/frontend/` directory.
  Delete it before committing.
- Use `wails3 build` directly: `wails3` is at `/c/Users/JohnCode/go/bin/` and
  `task` is not on PATH.
