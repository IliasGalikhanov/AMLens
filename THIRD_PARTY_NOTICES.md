# License and dependencies

AMLens source code is published under [MIT](LICENSE). Copyright: AMLens contributors. The project grew out of team work at HackAlem AI; subsequent development and container packaging took place after the competition.

The project license does not replace third-party licenses or apply to user financial data or hackathon organizers' materials. This repository does not contain `data.zip`, `starter.zip`, the original dataset or results derived from it.

License texts for libraries included in the Go binary (Linux/Windows amd64) and React browser bundle are preserved in [third_party](third_party/README.md), together with their versions. They were copied from installed packages without changing attribution.

Refresh the inventory after changing dependencies:

```sh
cd frontend
npm ci
cd ../backend
go mod download
cd ..
node scripts/collect-licenses.mjs
```

Before publishing, compare the inventory with `go.mod`, `go.sum` and `package-lock.json`. Build tools (Node.js, npm, TypeScript, Vite and the Go toolchain), Caddy and base Linux images carry their own licenses and notices in their distributions. Container builds copy `LICENSE` and `third_party` with the application.

The README and project history should preserve the prototype's team origin. Synthetic examples are generated programmatically; they are not anonymized copies of the original dataset.
