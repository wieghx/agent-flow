# Agent Flow Web (React)

Vite + React + TypeScript frontend for Agent Flow.

## Development

```bash
# Terminal 1: API (planner on :8082)
go run ./cmd/planner/ --ai-config=config/ai_config.yaml

# Terminal 2: Vite dev server (proxies API)
cd web && npm install && npm run dev
# http://localhost:5173
```

## Production

```bash
cd web && npm run build && npm start
# Serves dist/ on :3000, proxies API to API_URL
```

## Structure

```
src/
  api/         HTTP client
  hooks/       usePolling, useTaskSSE
  lib/paths.ts output & chapter URL helpers
  pages/       Chat, Tasks, Workflows, Monitor, NovelReader
  types/       API TypeScript interfaces
```