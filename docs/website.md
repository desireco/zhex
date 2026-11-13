# Zhex website and GitHub Pages

The website introduces Zhex, runs its examples at build time, and organizes language documentation into topic pages. Astro pages, layouts, styles, assets, and Markdown content live under `site/`; the Go helper in `tools/site-data` runs the actual interpreter and prepares the data Astro renders. The [published site](https://desireco.github.io/zhex/) is deployed by the [Pages workflow](https://github.com/desireco/zhex/actions/workflows/pages.yml).

## Build and preview

Install Go, Node 22.23.4 or newer (or Node 24), and the locked site dependencies, then build or preview from the repository root:

```sh
npm ci --prefix site
just site
just serve
```

Open [localhost:8000/zhex/](http://127.0.0.1:8000/zhex/). `_site/` and `site/data.json` are generated; edit sources instead. `just test` runs Go tests and the complete website build. `go vet ./...` checks Go source.

The Astro configuration sets the `/zhex` Pages base and emits stable `.html` URLs. `site/src/content/docs/` holds topic pages with `title`, `description`, `section`, and `order` frontmatter. Keep them consistent with `docs/initial-language.md`, `docs/getting-started.md`, and `docs/editors.md`. The legacy docs anchors and `/errors.html` catalog anchors remain available. `site/design/` ships verbatim, along with the example sources, roadmap, manifesto, and `llms.txt`.

## Build-time checks

The data helper executes every registered program in `examples/` with fixed input and arguments, including the two-file module example. It reads the source and output from the interpreter, highlights escaped source, validates the roadmap and manifesto, and reads the interpreter's diagnostic catalog. A broken program or an unexpected process status fails the build. Examples that deliberately call `console.exit(0)` are registered with an expected zero exit status; examples whose expected status is nonzero cannot be rendered through the successful-example pipeline. Example metadata is in `tools/site-data/main.go`. After Astro builds, `site/scripts/finish.mjs` copies public assets and checks local links and anchors, and `site/scripts/verify.mjs` checks rendered examples, downloads, and compatibility anchors. The site is static; it does not execute Zhex in the browser.

The root `ROADMAP.md` uses a title, an introductory paragraph, and Done, Next, and Future sections with single-line bullets. The data helper rejects missing sections and unsupported paragraph markup. Keep it aligned with the [project board](https://github.com/users/desireco/projects/8).

## Publishing

The Pages workflow installs locked npm dependencies, tests Go, builds the site, and uploads `_site/`. Pull requests build without publishing; pushes to `main` and manual runs on `main` deploy to the `github-pages` environment. Only the deploy job receives Pages and identity-token write permissions. Repository Settings → Pages must use GitHub Actions as its source. Check the workflow and deployment URL after pushing; roll back by pushing the intended source revision and letting the workflow publish it.
