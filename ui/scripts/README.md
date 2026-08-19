# Customer Dashboard UI Scripts

## Scripts

### `generate-api-types.js`
Generates TypeScript types from the Nuon OpenAPI spec into `client/types/nuon-oapi-v3.d.ts` using `openapi-typescript`.

## Package scripts using these files

- `bun run generate-api-types` runs `generate-api-types.js`

CSS is not built by a script here: `client/main.tsx` imports `client/styles.css`,
so Vite processes it through `postcss.config.js` (Tailwind + autoprefixer) as part
of `bun run dev` / `bun run build`. Ladle does the same via
`.ladle/components.tsx`.

## Usage

### Generate API types (production API, default)
```bash
bun run generate-api-types
```

### Generate API types from local API
```bash
NUON_API_URL=http://localhost:8081 bun run generate-api-types
```

### Generate API types from a local spec file
```bash
NUON_OPENAPI_SPEC_FILE=./path/to/spec.json bun run generate-api-types
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NUON_API_URL` | `https://api.nuon.co` | API URL to fetch the OpenAPI spec from |
| `NUON_OPENAPI_SPEC_FILE` | — | Local spec file path (takes precedence over `NUON_API_URL`) |
