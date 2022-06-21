<p align="center">
  <a href="https://www.gitpod.io">
    <img src="docs/logo.png" alt="rungp Logo" height="60" />
    <br />
    <strong>run-gp</strong>
  </a>
  <br />
  <span>Run local workspaces using the <code>.gitpod.yml</code></span>
</p>
<p align="center">
  <a href="https://gitpod.io/from-referrer/">
    <img src="https://img.shields.io/badge/Gitpod-ready--to--code-908a85?logo=gitpod" alt="Gitpod ready-to-code" />
  </a>
  <a href="https://www.gitpod.io/chat">
    <img src="https://img.shields.io/discord/816244985187008514" alt="Discord" />
  </a>
</p>

`run-gp` is a CLI tool for running workspaces based on a `.gitpod.yml` file locally on your machine. Using a local working copy it produces a workspace image, and starts that workspace. This provides an experience akin to a regular Gitpod workspace.

> **Warning**
> This is an experiment. When you find an issue, [please report it](https://github.com/gitpod-io/run-gp/issues/new?assignees=&labels=&template=bug_report.md&title=) so that we can improve this project.

## Features
- ✅ **Image Build**: `run-gp` produces a workspace image based on the `image` section in the `.gitpod.yml`. If no such section exists, `gitpod/workspace-full:latest` is used.
- ✅ **Browser Access**: by default we'll start [Open VS Code server](https://github.com/gitpod-io/openvscode-server) to provide an experience akin to a regular Gitpod workspace. This means that a `run-gp` workspace is accessible from your browser.
- ✅ **SSH Access**: if your user has an SSH key (`~/.ssh/id_rsa.pub` file) present, the run-gp workspace will sport an SSH server with an appropriate entry in authorized_keys. This means that you can just SSH into the `run-gp` workspace, e.g. from a terminal or using VS Code.
- ✅ VS Code extension installation: VS Code extensions specified in the `.gitpod.yml` will be installed when the workspace starts up. Those extensions are downloaded from [Open VSX](https://open-vsx.org), much like on gitpod.io.
- ✅ **Tasks** configured in the `.gitpod.yml` will run automatically on startup. 
- ✅ **Airgapped startup** so that other the image that's configured for the workspace no external assets need to be downloaded. It's all in the `run-gp` binary.
- ✅ **Auto-Update** which keeps `run-gp` up to date without you having to worry about it. This can be disabled - see the Config section below.
- ⚠️ **Docker-in-Docker** depends on the environment you use `run-gp` in. It does not work yet on MacOS and when `run-gp` is used from within a Gitpod workspace.
- ⚠️ **JetBrains Gateway support** also depends on the environment `run-gp` is used in. It is known NOT to work on arm64 MacOS.
- ⏳ **`gp` CLI** is coming in a future release.
- ❌ **Gitpod Prebuilds** are unsupported because this tool is completely disconnected from [gitpod.io](https://gitpod.io).
- ❌ **Gitpod Backups** are unsupported because this tool is completely disconnected from [gitpod.io](https://gitpod.io).

## Getting Started
1. Download the [latest release](https://github.com/gitpod-io/run-gp/releases/latest)
2. In a terminal, navigate to a directory which has a `.gitpod.yml` file, e.g. a Git working copy of your repository.
3. Run `run-gp`
4. Once the workspace is ready, open the URL displayed in the terminal.

## Configuration
`run-gp` does not have a lot of configuration settings, as most thinsg are determined by the `.gitpod.yml`. You can find the location of the configuration file using
```bash
run-gp config path
```

### Auto Update behaviour
By default `run-gp` will automatically update itself to the latest version. It does that by checking the GitHub releases of the run-gp repository for a new release.
To disable this behaviour run:
```bash
run-gp config set autoUpdate.enabled false
```

### Telemetry
By default `run-gp` will send anonymous telemetry. We never send identifiable details such as usernames, URLs or the like. You can review all data ever being transmitted [in the sources](https://github.com/gitpod-io/run-gp/blob/main/pkg/telemetry/telemetry.go#L84-L123). To disable telemetry run:
```bash
run-gp config set telemetry.enabled false
```