# Distribution & Release System

Complete distribution and release system for Frappe MCP Server.

## 📦 What Was Implemented

### 1. **Multi-Platform Build System** ✅

Updated `Makefile` with new targets:

```bash
make build-stdio-all    # Build for all platforms
make release            # Full release build (HTTP + STDIO, all platforms)
```

**Supported Platforms:**
- Linux AMD64
- Linux ARM64
- macOS Intel (AMD64)
- macOS Apple Silicon (ARM64)
- Windows AMD64

### 2. **GitHub Actions CI/CD** ✅

#### CI Workflow (`.github/workflows/ci.yml`)
- **Triggered on**: Push to main/develop, Pull Requests
- **Jobs**:
  - Test: Run tests with coverage
  - Lint: golangci-lint
  - Build: Cross-compile for all platforms
  - Security: Gosec security scanning

#### Release Workflow (`.github/workflows/release.yml`)
- **Triggered on**: Version tags (v*)
- **Automated Process**:
  1. Build binaries for all 5 platforms
  2. Create distribution packages (.tar.gz, .zip)
  3. Generate SHA256 checksums
  4. Create GitHub Release with all artifacts
  5. Auto-generate release notes
  6. Update install.sh with new version

### 3. **Installation Script** ✅

**File**: `install.sh`

**Features:**
- Auto-detects OS and architecture
- Downloads latest release from GitHub
- Installs to `~/.local/bin`
- Creates configuration directory
- Shows MCP client setup instructions

**Usage:**
```bash
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

### 4. **Comprehensive Documentation** ✅

All documentation in `/docs`:

- **`installation.md`** - Complete installation guide
  - Automated install script
  - Manual download instructions
  - Build from source
  - Platform-specific instructions
  - Troubleshooting

- **`releases.md`** - Release process documentation
  - How to create releases
  - Automated vs manual process
  - Version numbering
  - Rollback procedures

- **`quick-start.md`** - Updated with installation options
- **`index.md`** - Updated with quick install

## 🚀 How to Use

### For Users

**Install:**
```bash
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

**Or download from releases:**
https://github.com/varkrish/frappe-mcp-server/releases/latest

### For Maintainers

**Create a Release:**
```bash
# 1. Update CHANGELOG.md
vim CHANGELOG.md

# 2. Commit changes
git add CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main

# 3. Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 4. GitHub Actions automatically:
#    - Builds all binaries
#    - Creates release
#    - Uploads artifacts
#    - Generates checksums
```

**Test Builds Locally:**
```bash
# Test multi-platform builds
make clean
make build-stdio-all

# Full release build
make release
```

## 📋 Build Artifacts

Each release includes:

### Binaries
```
frappe-mcp-server-stdio-linux-amd64.tar.gz
frappe-mcp-server-stdio-linux-arm64.tar.gz
frappe-mcp-server-stdio-darwin-amd64.tar.gz
frappe-mcp-server-stdio-darwin-arm64.tar.gz
frappe-mcp-server-stdio-windows-amd64.zip
```

### Package Contents
- Binary (frappe-mcp-server-stdio)
- README with quick start
- env.example (configuration template)

### Checksums
- SHA256SUMS file for verification

## 🔄 Release Workflow

```
Developer                GitHub Actions              Users
   │                            │                       │
   ├─ Create tag (v1.0.0)       │                       │
   ├─ Push tag ────────────────►│                       │
   │                            ├─ Run tests            │
   │                            ├─ Build binaries       │
   │                            ├─ Create packages      │
   │                            ├─ Generate checksums   │
   │                            ├─ Create release       │
   │                            └─ Upload artifacts     │
   │                            │                       │
   │                            │        install.sh     │
   │                            │◄──────────────────────┤
   │                            │                       │
   │                            ├─ Serve latest binary─►│
   │                            │                       │
```

## 🧪 Tested

✅ All 5 platform binaries build successfully:
```
-rwxr-xr-x  9.6M frappe-mcp-server-stdio-darwin-amd64
-rwxr-xr-x  9.1M frappe-mcp-server-stdio-darwin-arm64
-rwxr-xr-x  9.6M frappe-mcp-server-stdio-linux-amd64
-rwxr-xr-x  9.1M frappe-mcp-server-stdio-linux-arm64
-rwxr-xr-x  9.7M frappe-mcp-server-stdio-windows-amd64.exe
```

## 📝 Next Steps

### To Create First Release:

1. **Verify GitHub repo settings:**
   - Repository name: `varkrish/frappe-mcp-server`
   - Ensure Actions are enabled
   - Verify GITHUB_TOKEN has write permissions

2. **Create first release:**
   ```bash
   git tag -a v1.0.0 -m "Initial release"
   git push origin v1.0.0
   ```

3. **Monitor GitHub Actions:**
   - Go to Actions tab
   - Watch the Release workflow
   - Verify artifacts are uploaded

4. **Test installation:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
   ```

### For Distribution:

Users can install via:
1. **Install script** (recommended)
2. **Download from releases** (manual)
3. **Build from source** (developers)

All methods documented in `/docs/installation.md`

## 🎯 Distribution Channels

- **Primary**: GitHub Releases (automated)
- **Install script**: One-line installation
- **Package managers**: Future
  - Homebrew formula
  - Snap package
  - apt/yum repositories

## 🔧 Maintenance

### Update Dependencies
```bash
go mod tidy
go mod verify
```

### Security Updates
```bash
# Run security scan
make security

# Update dependencies
go get -u ./...
go mod tidy
```

### Build Verification
```bash
# Clean build all platforms
make clean
make release

# Verify binaries
ls -lh bin/
```

## 📊 Metrics

After release, track:
- Download counts (GitHub Insights)
- Install script usage (if tracking added)
- Issue reports
- Platform popularity

## 🆘 Troubleshooting

### GitHub Actions Fails
1. Check workflow logs in Actions tab
2. Test locally: `make release`
3. Verify Go version compatibility

### Install Script Issues
1. Test locally first
2. Check download URLs
3. Verify release artifacts exist

### Binary Issues
1. Test on target platform
2. Check cross-compilation: `file bin/*`
3. Verify no CGO dependencies

## 📚 Documentation

Complete documentation at:
- [Installation Guide](docs/installation.md)
- [Release Process](docs/releases.md)
- [Quick Start](docs/quick-start.md)

## ✅ Summary

**Implemented:**
- ✅ Multi-platform build system (Makefile)
- ✅ GitHub Actions CI (testing, linting, security)
- ✅ GitHub Actions Release (automated releases)
- ✅ Installation script (one-line install)
- ✅ Comprehensive documentation
- ✅ Build verification (tested successfully)

**Ready for:**
- First release (just create and push a tag!)
- Users to install via script or releases
- Automated distribution on every tag

