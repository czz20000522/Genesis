# Desktop Distribution and Workbench Design

## Scope

Make the Windows desktop application self-contained, safely installable, manually updateable, and easier to operate as a local workbench. The application remains a shell over `genesisd`; it does not take ownership of kernel facts or provider context.

Current implementation note: release `0.1.6` temporarily installs a separate `genesisd.exe` to close the packaged-service gap. It does not satisfy this approved distribution contract and must not be the basis of the next release.

## Visual thesis

Genesis is a quiet local workbench: narrow navigation, warm-white canvas, graphite text, and teal reserved for the active action and healthy state. The working surface is the conversation and composer, not a welcome card. Plain scoped CSS remains the only styling strategy.

The radius scale is 4px for small controls, 8px for inputs and buttons, and 12px for major surfaces. Buttons use a 120ms transform/opacity transition and `:active` scale only; reduced-motion disables it.

## Distribution contract

- The NSIS installer records `InstallLocation` in its own uninstall registry key and uses it as the next install default. The user can still select a different directory.
- The selected installation directory exposes only `genesis-desktop.exe` and its uninstaller. The desktop starts the same executable with an internal `--genesisd-sidecar` argument; the child is hidden and remains lifecycle-owned by its parent.
- Uninstall deletes only the two installed executables, its shortcuts, and its own uninstall registry key. It never recursively removes `$INSTDIR`, WebView data, `~/.genesis`, or user-created files.
- The `genesisd` command and desktop child mode share one internal server bootstrap package so provider, ledger, and kernel behavior do not drift.

## Update contract

- Settings exposes an explicit “check for updates” action. No background service and no automatic update download are introduced.
- The desktop queries the latest release for `czz20000522/Genesis-Private`, compares semantic versions, and shows the release notes, installer size, and SHA-256 asset before the user starts installation.
- The installer is downloaded to a temporary file, checksum-verified against the published `.sha256` asset, then launched after Genesis exits. It reuses the installer’s recorded directory.
- Because the repository is private, the check requires a GitHub fine-grained personal access token with read access to this repository. The token is stored through the existing local protected credential store and is never returned to the frontend after saving.

## Connection and local-model states

- The normal desktop launch owns the bundled `genesisd`; external mode remains read-only attachment when `GENESIS_KERNEL_BASE_URL` is set.
- The connection indicator distinguishes `checking`, `connected`, and a concise actionable failure. The settings diagnostic view keeps the detailed reason and log path.
- Starting the local WSL model is explicit. While startup is in progress, the button is disabled and reports that Qwen is loading; it does not show a false failure while readiness polling continues.

## Acceptance

1. Reinstalling defaults to the prior selected directory, and a custom directory is not recursively removed by uninstall.
2. A packaged desktop starts its hidden same-executable kernel child and reports ready when its local configuration is valid.
3. Settings can check a private latest release with a configured token, reject a checksum mismatch, and launch only a verified installer after user confirmation.
4. The first viewport puts navigation, connection/model state, and a usable composer in a compact workbench layout without a centered decorative welcome card.
