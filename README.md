# zgx

**A portable, single-binary CLI that configures HP ZGX nano devices over SSH — discovery, key-based access, app installation, and ConnectX pairing — from one command line on Linux, macOS, or Windows.**

`zgx` is an independent, cross-platform reimplementation of the configuration
workflows in HP's [ZGX-Toolkit](https://github.com/HPInc/ZGX-Toolkit) VS Code
extension, rebuilt in Go as a self-contained binary. No VS Code, no Node
runtime — just one executable that finds your device on the network, sets up
SSH key access, installs a curated AI/ML stack, and wires up high-speed
ConnectX networking between devices.

> **Status — early / not yet hardware-verified.** Every command is implemented
> and unit-tested, the test suite is race-clean, and the security-sensitive
> surface has been through a multi-agent security review. The flows have **not
> yet been run end-to-end against a physical ZGX device.** Treat this as a
> `v0.x` preview: review the commands it will run (it shows them before
> executing) and use at your own risk until a hardware-verified release is cut.

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

### Download a binary (recommended)

Grab the archive for your OS/arch from the
[Releases page](https://github.com/AndersSol/zgx/releases), unpack it, and put
`zgx` on your `PATH`.

### Install with Go

```sh
go install github.com/AndersSol/zgx@latest
```

### Build from source

```sh
git clone https://github.com/AndersSol/zgx.git
cd zgx
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
  require explicit `ja` confirmation.
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

[**linkedin.com/in/anders.solstad**](https://www.linkedin.com/in/anders.solstad)

<img src="docs/linkedin-qr.svg" alt="LinkedIn QR — Anders Solstad" width="160" />
