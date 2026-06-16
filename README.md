# ZGX CLI

> The binary and command are named `zgx`. The repository is **zgx-cli** for discoverability.

**A portable, single-binary CLI that configures HP ZGX nano devices over SSH — discovery, key-based access, app installation, and ConnectX pairing — from one command line on Linux, macOS, or Windows.**

`zgx` is an independent, cross-platform reimplementation of the configuration
workflows in HP's [ZGX-Toolkit](https://github.com/HPInc/ZGX-Toolkit) VS Code
extension, rebuilt in Go as a self-contained binary. No VS Code, no Node
runtime — just one executable that finds your device on the network, sets up
SSH key access, installs a curated AI/ML stack, and wires up high-speed
ConnectX networking between devices. And because it's just commands, your own
**AI coding agent** can drive it — see [Use it with your AI agent](#use-it-with-your-ai-agent).

> **Status — hardware-verified, `v0.x`.** Discovery, SSH key setup, app install,
> health checks and mDNS registration have all been run end-to-end against a real
> NVIDIA DGX Spark (DGX OS 7.5). ConnectX pairing is implemented but still needs
> two-device verification. The test suite is race-clean and the security-sensitive
> surface went through a multi-agent security review. Still moving fast — it shows
> every remote command before running it, and you should pin a release for
> production use.

---

## Highlights

- **One binary, every desktop OS.** Pure-Go, no cgo — cross-compiles to
  Linux, macOS, and Windows (amd64 + arm64).
- **Zero-config discovery.** Finds ZGX devices on the local network via mDNS
  (`_hpzgx._tcp` / `_ssh._tcp`).
- **Safe SSH bootstrap.** Generates an ed25519 key, copies it to the device,
  and writes an `~/.ssh/config` alias — idempotently, with trust-on-first-use
  host-key verification and a fingerprint confirmation prompt.
- **Curated AI/ML app catalog.** 17 apps (PyTorch env, Ollama, Jupyter,
  Open WebUI, LangChain, …) installed in dependency order over SSH, with
  partial-failure reporting.
- **Transparent installs.** Shows the raw remote commands before running them
  and asks for confirmation before anything downloads-and-executes code.
- **High-speed pairing.** Discovers Mellanox/ConnectX NICs (`lshw`) and writes
  link-local netplan to pair devices.

---

## Install

### Homebrew (macOS & Linux) — recommended

```sh
brew install --cask AndersSol/tap/zgx
```

Auto-updates with `brew upgrade`; remove with `brew uninstall --cask zgx`.

### macOS — double-click installer

Download `zgx_<version>_darwin_universal.pkg` from the
[Releases page](https://github.com/AndersSol/zgx-cli/releases) and double-click
it. The installer is signed with an Apple Developer ID and **notarized**, so it
runs without any Gatekeeper warning and puts `zgx` on your `PATH`
(`/usr/local/bin/zgx`). Universal binary — works on both Apple Silicon and Intel.

### Download a binary

Grab the archive for your OS/arch from the
[Releases page](https://github.com/AndersSol/zgx-cli/releases), unpack it, and put
`zgx` on your `PATH`. The macOS binaries are signed and notarized too, so they
run out of the box.

### Install with Go

```sh
go install github.com/AndersSol/zgx-cli@latest
```

> `go install` produces a binary named `zgx-cli`; rename it to `zgx`, or just use the
> prebuilt release binaries (already named `zgx`).

### Build from source

```sh
git clone https://github.com/AndersSol/zgx-cli.git
cd zgx-cli
go build -o zgx .
```

Requires Go 1.26+. The build has no external system dependencies.

---

## Quick start

```sh
# 1. Find your device on the network
zgx discover

# 2. Set up SSH key access (you'll enter the device password once)
zgx connect zgx-ab12cd --user hp

# 3. See what you can install
zgx list

# 4. Install an app (dependencies are pulled in automatically)
zgx install jupyter-lab --host zgx-ab12cd

# 5. Check the device is reachable
zgx health zgx-ab12cd
```

After `connect`, the device is reachable as an SSH alias too:
`ssh zgx-ab12cd`.

---

## Use it with your AI agent

`zgx` is just commands, so your AI coding agent can drive your ZGX nano — no IDE
lock-in. Drop in the agent skill and ask in plain language: the agent runs the
read-only steps (`discover`, `health`, `verify`) itself and hands you the two
password-gated commands (`connect`, `install`) to run.

Built for **Claude Code** — the skill is plain Markdown, so any agent can read it:

```sh
mkdir -p ~/.claude/skills/zgx-cli
curl -fsSL https://anderssol.github.io/zgx-cli/zgx-cli.skill.md \
  -o ~/.claude/skills/zgx-cli/SKILL.md
```

Then say things like *"set up my ZGX nano for the first time"* or *"install the
jupyter + ollama stack on spark-7f2a and verify it"*. The skill ships the exact
command surface, the gotchas (the default user is `hp`, but a re-imaged DGX Spark
uses the name you created at first boot), and self-checking recipes that were
verified against real hardware and adversarially reviewed.
[Read the skill →](docs/zgx-cli.skill.md)

---

## Commands

| Command | What it does |
| --- | --- |
| `zgx discover` | Find ZGX devices on the network via mDNS. |
| `zgx connect <host>` | Generate an ed25519 key, install it on the device, write an SSH config alias, and verify key access. |
| `zgx list` | Print the curated app catalog (categories + apps). |
| `zgx install [app…] \| --all` | Install apps over SSH in dependency order. |
| `zgx verify [app…] \| --all` | Check whether apps are installed. |
| `zgx uninstall [app…] \| --all` | Remove apps (only the ones you name — shared dependencies are kept). |
| `zgx health <host>` | Verify SSH connectivity to a device. |
| `zgx dns-register <host>` | Register the device with Avahi for stable mDNS rediscovery. |
| `zgx pair <host>` | Configure ConnectX NICs (link-local netplan) for high-speed device pairing. |
| `zgx unpair <host>` | Remove the ConnectX pairing config. |
| `zgx pair-details <host>` | Show the device's ConnectX NICs and their IPs. |
| `zgx config add/list/remove` | Save and manage known devices. |

Run `zgx <command> --help` for flags. SSH commands share `--user` (default
`hp`), `--port` (default `22`), `--identity`, and `--known-hosts`.

---

## How it works

`zgx` is split into a front-end-agnostic **engine** (`internal/…` — discovery,
SSH, install orchestration, pairing, DNS, health) and a thin **CLI**
(`cmd/…`). This keeps the core testable without a UI and leaves the door open
for a TUI or native app on top of the same engine later.

- **Transport:** pure-Go SSH via `golang.org/x/crypto/ssh` — no shell-out to a
  system `ssh` binary, full control over exec/timeout/host-key.
- **mDNS:** [`brutella/dnssd`](https://github.com/brutella/dnssd).
- **App catalog:** embedded JSON (`go:embed`), with each app's install/verify/
  uninstall command and dependency list ported verbatim from the HP source so
  it stays diffable against upstream.

### Install ordering

When you install an app, `zgx` automatically pulls in its dependencies and
installs `base-system` first, in topological order (with cycle detection).
`uninstall`, by contrast, removes **only** the apps you name — it never
expands to shared dependencies that other apps may still need.

---

## Security

`zgx` runs remote shell commands and writes to sensitive local files, so it's
built defensively:

- **ed25519 keys** generated in-process; an existing private key is **never**
  overwritten, and key files are written `0600` / `0644`.
- **Host-key trust-on-first-use.** On first contact with an unknown host,
  `zgx` shows the ED25519 fingerprint and asks you to confirm **before** the
  password is sent. A changed key on a known host is always rejected (possible
  MITM). `InsecureIgnoreHostKey` is never used.
- **Transparent execution (no surprise code).** `install` prints the raw
  remote commands first and flags download-and-execute lines (`curl … | sh`,
  `curl … && bash …`) with a ⚠. `--all` and any plan containing such a command
  require explicit `yes` confirmation.
- **Input validation.** Values written into `~/.ssh/config` reject newlines and
  control characters (no SSH-directive injection); discovered NIC names and
  device identifiers are validated before they're interpolated into remote
  commands or config files.
- **No secrets on disk or in logs.** Passwords are read from a hidden prompt,
  sent only over the authenticated SSH channel, and never written to a file,
  log, or error message.

The repository carries no embedded secrets (verified with `gitleaks`).

---

## Configuration

Known devices are stored in `$XDG_CONFIG_HOME/zgx/config.json` (or
`~/.config/zgx/config.json`), written `0600`:

```sh
zgx config add zgx-ab12cd 192.168.1.50 --user hp
zgx config list
zgx config remove zgx-ab12cd
```

---

## Development

```sh
go build ./...     # compile
go vet ./...       # static checks
go test -race ./... # tests (race detector)
```

Releases are built and published by [GoReleaser](https://goreleaser.com) on
tag push (see `.github/workflows/release.yml`).

---

## License & attribution

`zgx` is released under the [MIT License](LICENSE).

It is an independent work based on HP's
[ZGX-Toolkit](https://github.com/HPInc/ZGX-Toolkit): the app-catalog data and
command semantics are ported from it. The full upstream X11/MIT permission
notice is preserved in [`NOTICE`](NOTICE). `zgx` is not affiliated with or
endorsed by HP.

---

## Author

Built by **Anders Solstad**. If this is useful and you'd like to follow along,
connect on LinkedIn:

[**linkedin.com/in/anders-solstad**](https://www.linkedin.com/in/anders-solstad)

<img src="docs/linkedin-qr.svg" alt="LinkedIn QR — Anders Solstad" width="160" />
