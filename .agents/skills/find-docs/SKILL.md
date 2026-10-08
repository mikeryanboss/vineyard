---
name: find-docs
description: >-
  Retrieve authoritative technical documentation, API references, configuration
  details, and examples. Use when answering technical questions or working with
  external libraries, frameworks, SDKs, APIs, CLI tools, or developer platforms,
  especially when documentation accuracy or version-specific behavior matters.
---

# Documentation Lookup

## Source Selection

- Use an available compatible Context7 interface or CLI, official documentation, or installed CLI help/source. Prefer primary sources matching the project's version.
- Reuse relevant documentation and validated library IDs already available in context. Do not repeat discovery solely because another lookup is needed.
- Install or update tooling only when necessary and permitted, not before every lookup. `npx ctx7@latest` may download and cache a package; it avoids a global install, not all installation.
- Keep queries specific and exclude credentials, personal data, and proprietary code.

## Context7 Workflow

1. Identify the library and version from the task and project configuration. Ask only if a material ambiguity remains.
2. Reuse a matching validated ID or one supplied by the user. Otherwise resolve it:

   ```bash
   ctx7 library <name> "<specific question>"
   ```

   Select the matching package and authoritative source; use documentation coverage and reputation to distinguish otherwise suitable results.

3. Query the resolved ID, not a bare library name:

   ```bash
   ctx7 docs /org/project "<specific question>"
   ```

   Use `/org/project/version` when that exact version is available.

4. After three Context7 calls for a question, switch to other authoritative sources rather than repeating unsuccessful lookups. This limit does not make an incomplete result sufficient evidence.

## Versions and Failures

- If the requested version is unavailable, consult versioned official documentation, release notes, or the installed source. Clearly label any version mismatch; do not present a nearby version as verification of the requested one.
- If Context7 is unavailable or quota-limited, use accessible official documentation. Explain any evidence gap; identify answers based only on model knowledge as unverified and potentially outdated.
- Authentication can use `CONTEXT7_API_KEY` or `ctx7 login` when needed and authorized. Do not expose secrets or require authentication when another authoritative source suffices.
- Cite the sources and versions used. If evidence is still insufficient, state what remains unknown rather than guessing.
