# Code protection notes

You cannot fully prevent reverse engineering. The goal is to raise the cost of casual copying.

## What you ship

| Part | Where it lives | Risk |
|------|----------------|------|
| **SaturX** | Go → one exe | No sources; reverse is possible but slower |
| **Viewerbot** | Python → PyInstaller exe | Bytecode can be extracted and partly decompiled |
| **UI (HTML/JS)** | Embedded in Go (`embed`) | Extractable from the exe, not shipped as a folder |

---

## 1. Go (SaturX)

**Already in the release build:**  
`-ldflags="-s -w"` strips symbols and debug info.

**Optional:**

- **Garble** — obfuscate names, strings, control flow:  
  `go install mvdan.cc/garble@latest`  
  then:  
  `garble -literals -tiny build -tags release -ldflags "..." -o SaturX.exe .`

---

## 2. Viewerbot (Python)

PyInstaller is not protection: the archive can be unpacked and `.pyc` decompiled.

Options:

- **PyArmor** (already in the build scripts): `pyarmor gen` before PyInstaller.
- **Cython** for critical paths.
- Never put passwords, API keys, or tokens in source. Keep license logic on the server.

---

## 3. Frontend (HTML/JS)

Static files are embedded with `//go:embed static`. Optional minify/obfuscate of JS/CSS before build.

---

## 4. Rules

- Do not store HMAC/API secrets in source; prefer server-side license checks.
- Do not push `.env`, keys, or passwords. Use `.gitignore`.
- Frequent binary updates plus server-side license checks make stolen copies less useful.

Bottom line: strip is already on. For more, use Garble (Go) and PyArmor (viewerbot). Nothing is unbreakable.
