# Custom PKGBUILD Repositories

This document describes the custom PKGBUILD repository feature implemented in yay, which allows users to configure and use custom package repositories alongside the official AUR.

## Overview

The custom repository feature enables users to:
- Maintain private PKGBUILD collections for proprietary software
- Create organization-specific package repositories
- Host custom packages that don't belong in the public AUR
- Develop and test packages locally before submitting to AUR
- Mirror or fork AUR packages with custom modifications

## Configuration

Custom repositories are configured in `~/.config/yay/config.json` under the `customRepos` array:

```json
{
  "customRepos": [
    {
      "name": "company-internal",
      "type": "git",
      "url": "https://git.company.com/pkgbuilds",
      "searchable": true,
      "priority": 1
    },
    {
      "name": "local-dev",
      "type": "local",
      "path": "/home/user/dev/pkgbuilds",
      "searchable": true,
      "priority": 2
    },
    {
      "name": "mirror-aur",
      "type": "http",
      "url": "https://my-server.com/aur-mirror",
      "searchable": false,
      "priority": 3
    }
  ]
}
```

### Repository Types

#### Local Repositories (`type: "local"`)
- Point to local filesystem paths
- Expected structure: `repo-root/package-name/PKGBUILD`
- Immediate availability, no network dependency

#### Git Repositories (`type: "git"`)
- Clone/update git repositories containing PKGBUILDs
- Support for authentication (SSH keys, tokens)
- Cached locally for offline access

#### HTTP Repositories (`type: "http"`)
- Simple HTTP-served directory structures
- Lightweight alternative to full AUR infrastructure
- Compatible with static file hosting (nginx, Apache, GitHub Pages)

### Repository Structure

```
repository-root/
├── package-a/
│   ├── PKGBUILD
│   ├── .SRCINFO
│   └── [additional files]
├── package-b/
│   ├── PKGBUILD
│   ├── .SRCINFO
│   └── [patches, sources, etc.]
└── [optional] packages.json  # metadata for faster searching
```

## CLI Commands

### Repository Management

```bash
# Add a custom repository
yay --repo-add <name> <type> <url/path> [options]

# Remove a custom repository
yay --repo-remove <name>

# List all custom repositories
yay --repo-list

# Update all custom repositories
yay --repo-update
```

### Examples

```bash
# Add a local repository
yay --repo-add local-dev local /home/user/dev/pkgbuilds

# Add a git repository
yay --repo-add company-internal git https://git.company.com/pkgbuilds

# Add an HTTP repository
yay --repo-add mirror-aur http https://my-server.com/aur-mirror

# List repositories
yay --repo-list

# Update repositories
yay --repo-update
```

## Integration with Existing Commands

### Search (`yay -Ss <query>`)
- Includes results from configured custom repositories
- Clearly indicates source repository in output
- Respects priority ordering

### Install (`yay -S <package>`)
- Checks custom repositories before falling back to AUR
- Supports repository-specific installation: `yay -S repo/package`

### Information (`yay -Si <package>`)
- Displays repository source and metadata
- Shows if package exists in multiple repositories

## Security Considerations

- Add configuration option to restrict custom repositories to trusted sources
- Warn users when installing from non-AUR sources
- Implement signature verification for HTTP repositories
- Sandboxed builds by default (existing yay behavior)

## Example Use Cases

### Company Development
```bash
# Search includes both AUR and company packages
yay -Ss company-tool

# Install from specific repository
yay -S company-internal/proprietary-software
```

### Local Development
```bash
# Test local PKGBUILD before AUR submission
yay -S local-dev/my-new-package
```

### Custom AUR Mirror
```bash
# Use organization's AUR mirror with modifications
yay -Ss modified-package
```

## Backward Compatibility

- Default behavior unchanged when no custom repositories configured
- Existing configuration files remain valid
- AUR remains the primary source unless explicitly overridden

## Performance

- Cache repository metadata locally
- Implement incremental updates for git repositories
- Parallel searching across repositories

## User Experience

- Clear indication of package sources in output
- Intuitive error messages for repository access issues
- Optional repository management commands

## Benefits

- Maintains yay's unified interface while adding flexibility
- Enables enterprise and development use cases
- Strengthens Arch Linux ecosystem by supporting diverse packaging needs
- Reduces need for complex workarounds and fragmented tooling

This feature makes yay more versatile while preserving its core strengths and maintaining compatibility with existing workflows.
