# Legacy reference code

`legacy/matrixhub-ai/` is **imported reference code only**.

- **Do not extend it directly.**
- It is a separate Go module and is excluded from this repository's build and
  tests.
- Useful modules have been refactored into clean implementations under
  `internal/`.

See [`../docs/refactor-from-matrixhub-ai.md`](../docs/refactor-from-matrixhub-ai.md)
for what was extracted, what was intentionally left out, and where future
registry features may go.
