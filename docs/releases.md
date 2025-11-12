# Release Process

How to create and publish releases for Frappe MCP Server.

## Automated Release Process

Releases are fully automated using GitHub Actions. When you push a version tag, the CI/CD pipeline automatically builds binaries for all platforms and creates a GitHub release.

## Creating a New Release

### 1. Prepare the Release

Update the changelog:

```bash
# Edit CHANGELOG.md
vim CHANGELOG.md
```

Add release notes under a new version section:

```markdown
## [1.0.0] - 2025-11-13

### Added
- New feature X
- Support for Y

### Fixed
- Bug Z

### Changed
- Improved performance of A
```

Commit the changes:

```bash
git add CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main
```

### 2. Create and Push the Tag

```bash
# Create a version tag (use semantic versioning)
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### 3. Automated Build Process

Once the tag is pushed, GitHub Actions automatically:

1. **Runs CI tests** - Ensures code quality
2. **Builds binaries** for all platforms:
   - `frappe-mcp-server-stdio-linux-amd64.tar.gz`
   - `frappe-mcp-server-stdio-linux-arm64.tar.gz`
   - `frappe-mcp-server-stdio-darwin-amd64.tar.gz`
   - `frappe-mcp-server-stdio-darwin-arm64.tar.gz`
   - `frappe-mcp-server-stdio-windows-amd64.zip`
3. **Creates packages** with:
   - Binary
   - README
   - Configuration template
4. **Generates checksums** (SHA256SUMS)
5. **Creates GitHub Release** with:
   - All platform binaries
   - Checksums file
   - Auto-generated release notes
6. **Updates install.sh** with the new version

### 4. Verify the Release

1. Go to [Releases](https://github.com/varkrish/frappe-mcp-server/releases)
2. Check that all binaries are attached
3. Test the install script:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
   ```
4. Verify binary works:
   ```bash
   frappe-mcp-server-stdio --version
   ```

## Manual Release (Emergency)

If GitHub Actions is unavailable, you can create releases manually:

### Build All Binaries

```bash
# Clean and build for all platforms
make clean
make release

# Binaries will be in ./bin/
ls -lh bin/
```

### Create Archives

```bash
cd bin

# Linux AMD64
tar czf frappe-mcp-server-stdio-linux-amd64.tar.gz \
  frappe-mcp-server-stdio-linux-amd64

# Linux ARM64
tar czf frappe-mcp-server-stdio-linux-arm64.tar.gz \
  frappe-mcp-server-stdio-linux-arm64

# macOS Intel
tar czf frappe-mcp-server-stdio-darwin-amd64.tar.gz \
  frappe-mcp-server-stdio-darwin-amd64

# macOS Apple Silicon
tar czf frappe-mcp-server-stdio-darwin-arm64.tar.gz \
  frappe-mcp-server-stdio-darwin-arm64

# Windows
zip frappe-mcp-server-stdio-windows-amd64.zip \
  frappe-mcp-server-stdio-windows-amd64.exe
```

### Generate Checksums

```bash
sha256sum *.tar.gz *.zip > SHA256SUMS
```

### Create GitHub Release

1. Go to [New Release](https://github.com/varkrish/frappe-mcp-server/releases/new)
2. Choose the tag
3. Add release title: "v1.0.0"
4. Add release notes
5. Upload all archives and SHA256SUMS
6. Publish release

## Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- **Major** (v2.0.0): Breaking changes
- **Minor** (v1.1.0): New features, backwards compatible
- **Patch** (v1.0.1): Bug fixes, backwards compatible

### Examples

- `v1.0.0` - First stable release
- `v1.1.0` - Added new MCP tools
- `v1.1.1` - Fixed connection bug
- `v2.0.0` - Changed API structure (breaking)

## Pre-releases

For testing before official release:

```bash
# Create pre-release tag
git tag -a v1.1.0-beta.1 -m "Beta release v1.1.0-beta.1"
git push origin v1.1.0-beta.1
```

Mark as "Pre-release" in GitHub release interface.

## Rollback

If a release has critical issues:

### 1. Delete the Release

```bash
# Delete remote tag
git push --delete origin v1.0.0

# Delete local tag
git tag -d v1.0.0
```

### 2. Delete GitHub Release

1. Go to Releases page
2. Click on the problematic release
3. Click "Delete"

### 3. Create Fixed Release

Fix the issue, then create a new patch release (e.g., v1.0.1).

## Release Checklist

Before tagging a release, ensure:

- [ ] All tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] CHANGELOG.md is updated
- [ ] Documentation is up to date
- [ ] Version tag follows semantic versioning
- [ ] Tag message is descriptive

After release:

- [ ] Verify all binaries are attached
- [ ] Test installation script
- [ ] Test binary on at least one platform
- [ ] Announce on relevant channels
- [ ] Update documentation if needed

## Continuous Deployment

The install script always fetches the **latest** release. When you create a new release, users automatically get the new version when they run:

```bash
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

## Build Artifacts

Each release includes:

### Binaries
- Cross-compiled for 5 platforms
- Statically linked (no dependencies)
- Optimized with `-ldflags "-s -w"`

### Archives
- `.tar.gz` for Linux/macOS
- `.zip` for Windows
- Includes binary + README + config template

### Checksums
- `SHA256SUMS` file
- Verifiable with: `shasum -a 256 -c SHA256SUMS`

## Troubleshooting

### GitHub Actions Fails

1. Check workflow logs in Actions tab
2. Common issues:
   - Go version mismatch
   - Build errors (run `make release` locally first)
   - Permission issues (check GITHUB_TOKEN)

### Binary Not Working

1. Test locally first: `make build-stdio`
2. Check cross-compilation: `make build-stdio-all`
3. Verify on target platform before releasing

### Install Script Issues

1. Test install script locally
2. Update VERSION in install.sh manually if auto-update fails
3. Check download URLs are correct

## Support

- **Issues**: [GitHub Issues](https://github.com/varkrish/frappe-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/varkrish/frappe-mcp-server/discussions)

