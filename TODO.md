# Feature Request: Support for Custom PKGBUILD Repositories

## Summary
Add support for configuring custom PKGBUILD repositories alongside AUR, allowing users to maintain private or organization-specific package collections that integrate seamlessly with yay's search and installation workflow.

## Motivation
Currently, yay is tightly coupled to the official AUR infrastructure. While this ensures consistency and security, it limits flexibility for users who want to:

- Maintain private PKGBUILD collections for proprietary software
- Create organization-specific package repositories
- Host custom packages that don't belong in the public AUR
- Develop and test packages locally before submitting to AUR
- Mirror or fork AUR packages with custom modifications

Users currently work around this limitation by:
- Creating wrapper scripts around yay
- Manually managing separate PKGBUILD directories
- Using complex build systems that bypass yay entirely

## Proposed Solution

### Configuration Format
Extend `~/.config/yay/config.json` to support custom repositories:

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

**Local repositories (`type: "local"`):**
- Point to local filesystem paths
- Expected structure: `repo-root/package-name/PKGBUILD`
- Immediate availability, no network dependency

**Git repositories (`type: "git"`):**
- Clone/update git repositories containing PKGBUILDs
- Support for authentication (SSH keys, tokens)
- Cached locally for offline access

**HTTP repositories (`type: "http"`):**
- Simple HTTP-served directory structures
- Lightweight alternative to full AUR infrastructure
- Compatible with static file hosting (nginx, Apache, GitHub Pages)

### Expected Repository Structure
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

### Integration with Existing Commands

**Search (`yay -Ss <query>`):**
- Include results from configured custom repositories
- Clearly indicate source repository in output
- Respect priority ordering

**Install (`yay -S <package>`):**
- Check custom repositories before falling back to AUR
- Support repository-specific installation: `yay -S repo/package`

**Information (`yay -Si <package>`):**
- Display repository source and metadata
- Show if package exists in multiple repositories

### Security Considerations
- Add configuration option to restrict custom repositories to trusted sources
- Warn users when installing from non-AUR sources
- Implement signature verification for HTTP repositories
- Sandboxed builds by default (existing yay behavior)

## Example Use Cases

1. **Company Development:**
   ```bash
   # Search includes both AUR and company packages
   yay -Ss company-tool
   
   # Install from specific repository
   yay -S company-internal/proprietary-software
   ```

2. **Local Development:**
   ```bash
   # Test local PKGBUILD before AUR submission
   yay -S local-dev/my-new-package
   ```

3. **Custom AUR Mirror:**
   ```bash
   # Use organization's AUR mirror with modifications
   yay -Ss modified-package
   ```

## Implementation Considerations

### Backward Compatibility
- Default behavior unchanged when no custom repositories configured
- Existing configuration files remain valid
- AUR remains the primary source unless explicitly overridden

### Performance
- Cache repository metadata locally
- Implement incremental updates for git repositories
- Parallel searching across repositories

### User Experience
- Clear indication of package sources in output
- Intuitive error messages for repository access issues
- Optional repository management commands (`yay --repo-add`, `yay --repo-list`)

## Alternative Approaches Considered

1. **Wrapper Scripts:** Users currently implement this functionality externally, but it requires maintaining separate tooling
2. **Fork yay:** Creates fragmentation and maintenance burden
3. **Different AUR Helper:** Switching tools loses yay's established feature set and community

## Related Issues
- Users frequently request support for private/custom package sources
- Organizations need better package management for internal tools
- Developers want smoother workflows for package development

## Benefits
- Maintains yay's unified interface while adding flexibility
- Enables enterprise and development use cases
- Strengthens Arch Linux ecosystem by supporting diverse packaging needs
- Reduces need for complex workarounds and fragmented tooling

This feature would make yay more versatile while preserving its core strengths and maintaining compatibility with existing workflows.



$ yay -Sy
:: Synchronizing package databases...
 core is up to date
 extra is up to date  
 multilib is up to date
:: Updating custom repositories...
 company-internal: Updating... done
 local-dev: Updating... done

 {
  "name": "company-internal",
  "type": "git", 
  "url": "https://git.company.com/pkgbuilds",
  "branch": "main",           // opcjonalnie
  "auth": {                   // opcjonalnie
    "type": "ssh_key"         // albo "token", "basic"
  }
}