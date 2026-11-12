# Zhex website and GitHub Pages

The website introduces Zhex and shows runnable examples. Its source is three HTML templates, CSS, and an SVG icon in `site/`; no JavaScript or package installation is required.

The GitHub Pages address is [desireco.github.io/zhex](https://desireco.github.io/zhex/). The repository is configured to publish through GitHub Actions. A successful deployment appears in the repository's [Pages workflow](https://github.com/desireco/zhex/actions/workflows/pages.yml).

The design uses warm ivory, forest green, a small lime accent, large type, and short explanations. The landing page introduces four examples and offers a local getting-started path. The examples page uses native disclosure elements; on narrow screens, content forms one column and only code scrolls horizontally. Code is the subject, so no decorative imagery or external font dependencies are needed.

## Build and preview

From the repository root:

```sh
go run ./tools/build-site
python3 -m http.server 8000 --directory _site
```

Open [localhost:8000](http://localhost:8000/). The generator accepts `-out directory` to select a different output directory. `_site/` is generated content; edit source files instead. The build produces `index.html`, `examples.html`, `roadmap.html`, a public copy of `ROADMAP.md`, styles, the icon, and public downloads under `examples/`.

## Keep examples accurate

The build reads the actual `.zhex` files in `examples/` and executes them through the Go interpreter using fixed sample input. Both the displayed source and output therefore come from the working implementation. A broken example fails the website build. Example metadata and sample input are declared in `tools/build-site/main.go`.

Both templates receive `HeroSource`, `HeroInput`, `HeroOutput`, and `Examples`. Each example has `ID`, `Title`, `Description`, `Source`, `Input`, `Output`, `Command`, and `SourceURL`. Source links point to the copied public `.zhex` files. Example content is escaped by Go's `html/template`; it must not be converted to trusted HTML.

Run `go test ./...` and `go vet ./...` after changes. Generator tests check source/output consistency, HTML escaping, required assets, local links and anchors, and failure behavior. The browser interface is a static presentation of examples, not an in-browser interpreter.

## Publishing

The [Pages workflow](../.github/workflows/pages.yml) tests and builds the site on pull requests. Pushes to `main` and manual workflow runs from `main` also upload the static artifact and deploy it to the `github-pages` environment. Action versions are pinned to verified commit SHAs. The build job has read-only repository permissions; only the deployment job receives Pages and identity-token write permissions.

In repository **Settings → Pages**, the build source must be **GitHub Actions**. No branch containing generated files is needed. Deployment publishes only `_site/`, including the six downloadable example programs, rather than the repository's internal planning documents or interpreter source tree. The repository's private visibility still governs access to clone commands and repository links; it does not restrict access to the published Pages site or its example downloads.

If Pages reports that the account plan does not support a private repository, keep the repository private and resolve the plan or select another hosting arrangement. Changing repository visibility is a separate decision. GitHub documents Pages availability for public repositories on Free plans and private repositories on eligible paid plans in [Using custom workflows with GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).

To update or roll back the site, push the intended source revision to `main` and let the workflow publish it. The workflow status and deployment URL are the source of truth for whether a version is live.

## Keep the roadmap current

Edit the root `ROADMAP.md` to update both the repository roadmap and the public roadmap page. It uses a title, an introductory paragraph, and three sections: Done, Next, and Future. Each section has a short summary and at most four or five single-line bullets. Use plain text inside paragraphs and bullets; the site escapes it as text. The generator rejects missing sections and unsupported paragraph markup. Keep delivery dates out until they are agreed, and move work to Done only when implementation and validation are complete.
