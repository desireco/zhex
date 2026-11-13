# How programming languages introduce themselves

Research date: November 12, 2026. This survey examines official homepages and the learning paths they link to. It covers three Zhex influences and five languages with different audiences. It studies presentation and onboarding; it does not independently validate their performance or adoption claims.

The most useful pattern for Zhex is a short promise followed by visible code, an observable result, and a clear next action. Our current page already has that structure. The biggest obstacle is access: visitors can read our public examples, but running the interpreter requires access to the private repository.

## What the sites show

The observations in this table describe the inspected pages. The final column is an interpretation for Zhex.

| Language | Opening message and evidence | First learning path | Lesson for Zhex |
| --- | --- | --- | --- |
| **Factor** | Leads with concatenative programming and implementation features. A record-and-sequence example selects the oldest person. Libraries, standalone deployment, binary downloads, documentation, and community links support the practical claims. | Recommends a binary download, then a listener or graphical environment and guided learning material. [Homepage](https://factorcode.org/), [learning guide](https://concatenative.org/wiki/view/Factor/Learning). | A useful data transformation can explain composition. Define unfamiliar vocabulary beside the example; the many competing learning links may ask too much of a first-time visitor. |
| **Mirth** | Its project README identifies a strongly typed concatenative language under development, names its influences, and immediately supplies a small hello-world program. It also lists editor integrations. | Repository navigation exposes examples and tutorials. The inspected GitHub README points to Codeberg as the main repository and warns that mirrors may lag. [Project README](https://github.com/mirth-lang/mirth). | Explicit experimental status and small working examples fit Zhex's stage. Explain where authoritative documentation and source live. |
| **8th** | Leads with cross-platform delivery and built-in facilities. GUI, encryption, SQLite, REST, and real application links make the intended work concrete. Its separate samples page progresses from hello-world to CGI and a database-building script. | A prominent acquisition link leads toward its product editions; a free edition is described alongside paid versions. Manuals, samples, and a forum provide other entry points. [Homepage](https://8th-dev.com/), [samples](https://8th-dev.com/samples.html). | Link a capability to a task someone can recognize. Broader application claims become credible when a working application accompanies them. |
| **Go** | Leads with simple, secure, scalable systems, then presents four supporting points, download/get-started actions, and company case studies. Its navigation separates learning, reference documentation, packages, and community. | The learning page offers installation, an interactive tour, selected tutorials, and domain-specific learning paths. [Homepage](https://go.dev/), [learning page](https://go.dev/learn/). | Give visitors distinct routes to try code and look up rules. Zhex can use tested examples as evidence while it has no comparable production history. |
| **Rust** | Connects reliability and efficiency to three groups: performance, reliability, and productivity. Four application areas include CLI tools and WebAssembly. Production use and contribution paths appear further down. | Getting started offers an online playground before installation, then introduces Rustup, Cargo, a generated project, and its output. [Homepage](https://rust-lang.org/), [getting started](https://rust-lang.org/learn/get-started/). | A few application categories are easier to scan than a feature inventory. Each category should lead to code or be explicitly marked as a future direction. |
| **Gleam** | Uses friendly language and a tiny greeting, then pairs claims with examples of concurrency, package use, and a compiler diagnostic. Tooling and interoperability receive concrete demonstrations. | The primary trial action opens an interactive tour. Its introduction says code is editable and runs locally in the browser through JavaScript compilation, with output and diagnostics visible. [Homepage](https://gleam.run/), [tour](https://tour.gleam.run/). | Showing one useful error can make type checking understandable. A browser trial is a valuable eventual destination for Zhex's WASM work. |
| **Zig** | Opens with a general-purpose language and toolchain, then explains a few design choices. Its prominent example tokenizes and parses numbers, manages an allocation, and tests the result; the command and successful output are shown. Community and foundation information explain project support. | Getting started covers releases, installation, editor support, project creation, execution, and expected output. It also acknowledges documentation limits. [Homepage](https://ziglang.org/), [getting started](https://ziglang.org/learn/getting-started/). | A small practical task with a test can carry more meaning than several claims. Keep installation details on a dedicated page as they grow. |
| **Ruby** | Leads with programmer affinity and immediately shows text manipulation, iteration, collection operations, and a greeting object. Trial links sit near examples; ecosystem discussion and named voices provide further context. | The short tutorial begins in IRB and explains expression results, printing, arithmetic, and named values through small experiments. [Homepage](https://www.ruby-lang.org/en/), [short tutorial](https://www.ruby-lang.org/en/documentation/quickstart/). | Familiar text operations are a good way to introduce rich values. Small edits with visible results suit the planned Zhex REPL. |

Mirth limitation: the Codeberg repository and candidate standalone sites could not be read through the research tool. The Mirth observations above apply to the project's explicitly identified GitHub mirror. No installation, browser tour, or example from these other languages was executed during this survey.

## Recommendations for Zhex

These are proposals based on the survey and the current [landing page source](../site/src/pages/index.astro), [README](../README.md), and [roadmap](../ROADMAP.md).

1. **Keep the first screen focused on readable composition.** Retain the short headline, one sentence, and a complete example. Explain that a word is a function and that a flow passes its result onward. Lead with behavior a visitor can inspect; leave the ancestry and design research one link away.
2. **Make access clear at the action.** While the repository is private, make reading examples the main public action. Put the repository-access requirement beside any local-run link. A future public binary download would need an explicit release and distribution decision. A browser playground can follow the WASM milestone once compilation and host I/O work.
3. **Show a transformation that reveals Zhex's distinctive feature.** The existing greeting demonstrates input; the composition example demonstrates a checked flow. Put the small composition example near the introduction, or give it a direct example link. Include input and exact output. Later, let a real streaming text utility demonstrate practical use.
4. **Use evidence appropriate to the project's stage.** The six runnable examples, source-level errors, test catalog, and editor support are useful proof today. Keep native performance, modules, WASM, and application libraries under their roadmap statuses. Add measurements and application stories as they become available.
5. **Keep navigation to four clear destinations.** Examples, Get started, Roadmap, and GitHub already fit. As documentation grows, give the getting-started page a short path into the language reference and editor setup. Put the longer history and implementation discussion deeper in the documentation.

## Proposed homepage copy and outline

This is a content proposal, not a website change. The current visual direction—large text, short sections, and code beside its result—can stay.

**Opening**

> Small words. Clear programs.
>
> Zhex is an experimental Forth-inspired language. Write small functions, give values clear names, and compose them through checked flows.

Follow with one complete composition example, its output, and a short explanation: “A word is a function. A flow passes each result to the next word.” Use **Read the examples** as the primary action. Label the local trial with its repository-access requirement.

**Try a small program**

Link to four existing examples: composition, console input, FizzBuzz, and palindrome. Give each a one-line description and preserve the runnable source and exact output.

**What works today**

> The Go interpreter runs typed words, local values, decisions, loops, and console input and output. Programs are checked before they run.

Link to the short guide and editor setup. One actual checker diagnostic could later demonstrate what “checked” means, using output generated by a test.

**Where it goes next**

> Next: interactive execution, explicit modules, and a useful text utility. The longer-term plan includes native compilation and WebAssembly.

Link to the roadmap. Keep games, web services, HTML processing, and database software in the ambitions section until working examples support a more specific claim.

## Keeping this research useful

Treat homepage copy as a small experiment. Ask a reader to explain the shown program, identify what runs today, and find the next command or example. Record where they hesitate before adding more copy. Recheck the cited pages when revisiting their current navigation or onboarding; these sites will change.
