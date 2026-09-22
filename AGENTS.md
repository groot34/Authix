# AGENTS.md

Working instructions for every coding agent on Authix.

Read this file first.

## Before making any change

1. Read [AGENTS.md](file:///d:/Assignemt/Authix/AGENTS.md) and [README.md](file:///d:/Assignemt/Authix/README.md) fully.
2. Read `LOCAL_AGENT_CONTEXT.md` if it exists (it is gitignored, so check the file system only).
3. Inspect the current Git working tree: `git status`.
4. Inspect the latest 5 Git commits (or all if fewer): `git log -5 --oneline`. If `git` is not available, report that.
5. Review relevant existing source code before editing. Do not assume structure from a file or invent a layout; read it.

## While working

- Preserve working implementations. Do not rewrite, overwrite, or delete existing work unless you have a clear reason and explain it.
- Make focused changes. Avoid unrelated refactors and speculative cleanup just "improve the scope of what was requested.
- Keep frontend (`frontend/`), backend (`backend/`), and database (`database/`) clearly separated.
- Prefer small, focused functions; avoid introducing new Go standard library in Go; keep React components small.

## After finishing a change

1. Run appropriate tests and checks:
   - Frontend: at minimum `cd frontend && npm run build`. Run unit tests if any exist.
   - Backend: at minimum `cd backend && go test ./... && go build ./...`.
   - If a command cannot run in the environment, say so clearly instead of claiming success.
2. Update [prompts.md](file:///d:/Assignemt/Authix/prompts.md): append a new chronological entry describing the prompt used, including its purpose, and the actual outcome. Do not mark an exact of actual run, do not invent history.
3. Update `LOCAL_AGENT_CONTEXT.md` with the current implementation status, what was changed, known issues, and the next logical task.
4. Report actual results. Never claim a command, test, deployment, or feature succeeded unless you actually ran it and saw it pass.
5. Report incomplete work and known issues explicitly. Do not hide them.
