#!/bin/bash
set -e

INSTALL_DIR="$HOME/.cargo/bin"
RUNTIME_DIR="$HOME/otter-runtime"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "======================================"
echo "otter Build and Installation Script"
echo "======================================"
echo ""

# Parse command line arguments
SKIP_BUILD=false
SKIP_INIT=false
SKIP_ENVS=false
OPTIONAL_ENVIRONMENT=""
SKIP_R_PACKAGES=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-init)
            SKIP_INIT=true
            shift
            ;;
        --skip-envs)
            SKIP_ENVS=true
            shift
            ;;
        --with-snakemake)
            OPTIONAL_ENVIRONMENT="snakemake"
            shift
            ;;
        --with-extra)
            OPTIONAL_ENVIRONMENT="extra"
            shift
            ;;
        --skip-r-packages)
            SKIP_R_PACKAGES=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-build        Skip building binaries"
            echo "  --skip-init         Skip initializing runtime directory"
            echo "  --skip-envs         Skip creating the required otter-core environment"
            echo "  --with-snakemake    Also create the optional otter-snakemake environment"
            echo "  --with-extra        Also create the optional otter-extra environment"
            echo "  --skip-r-packages   Deprecated no-op (R packages are no longer installed)"
            echo "  --dry-run           Show what would be done without executing"
            echo "  --help              Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

if [ "$DRY_RUN" = true ]; then
    echo "DRY RUN MODE - No actual changes will be made"
    echo ""
fi

# ============================================================================
# Step 1: Build and Install Binaries
# ============================================================================
if [ "$SKIP_BUILD" = false ]; then
    echo "=== Step 1: Build and Install Binaries ==="

    if [ "$DRY_RUN" = false ]; then
        # Build otter
        echo "Building otter..."
        cd "$PROJECT_ROOT"
        CGO_ENABLED=0 go build -ldflags="-s -w" -o otter .
        mkdir -p "$INSTALL_DIR"
        cp otter "$INSTALL_DIR/"
        echo "✓ otter installed to $INSTALL_DIR/"

        # Build enva
        if [ -f "$PROJECT_ROOT/enva/Cargo.toml" ]; then
            echo "Building enva..."
            cd "$PROJECT_ROOT/enva"
            cargo build --release
            cp target/release/enva "$INSTALL_DIR/"
            echo "✓ enva installed to $INSTALL_DIR/"
        fi
    else
        echo "[DRY RUN] Would build and install:"
        echo "  - otter → $INSTALL_DIR/otter"
        echo "  - enva → $INSTALL_DIR/enva"
    fi

    echo ""
else
    echo "=== Step 1: Skipped (--skip-build) ==="
    echo ""
fi

# ============================================================================
# Step 2: Initialize Runtime Directory
# ============================================================================
if [ "$SKIP_INIT" = false ]; then
    echo "=== Step 2: Initialize Runtime Directory ==="

    if [ "$DRY_RUN" = false ]; then
        mkdir -p "$RUNTIME_DIR"
        cd "$RUNTIME_DIR"

        # Check if already initialized
        if [ -f "config/otter.yaml" ] || [ -d "rules" ]; then
            echo "⚠ Runtime directory already initialized"
            echo "  To re-initialize, remove $RUNTIME_DIR and run again"
        else
            "$PROJECT_ROOT/otter" init .
            echo "✓ Runtime directory initialized at $RUNTIME_DIR/"
        fi
    else
        echo "[DRY RUN] Would initialize runtime directory at $RUNTIME_DIR/"
    fi

    echo ""
else
    echo "=== Step 2: Skipped (--skip-init) ==="
    echo ""
fi

# ============================================================================
# Step 3: Create Conda Environments
# ============================================================================
if [ "$SKIP_ENVS" = false ]; then
    echo "=== Step 3: Create Conda Environments ==="

    # Check if enva is available
    ENVA_PATH="$INSTALL_DIR/enva"
    if [ ! -f "$ENVA_PATH" ]; then
        ENVA_PATH="$PROJECT_ROOT/enva/target/release/enva"
    fi

    if [ ! -f "$ENVA_PATH" ]; then
        echo "⚠ enva not found, skipping conda environment creation"
        echo "  Build enva first or use --skip-envs"
    else
        if [ "$DRY_RUN" = false ]; then
            cd "$RUNTIME_DIR"

            # Core is the only required environment. Optional compatibility layers
            # are opt-in because canonical runs use Craftmake directly.
            case "$OPTIONAL_ENVIRONMENT" in
                snakemake)
                    "$ENVA_PATH" create --core --snakemake
                    ;;
                extra)
                    "$ENVA_PATH" create --core --extra
                    ;;
                *)
                    "$ENVA_PATH" create --core
                    ;;
            esac
            echo "✓ Core Conda environment created"
        else
            echo "[DRY RUN] Would create conda environments using enva"
        fi
    fi

    echo ""
else
    echo "=== Step 3: Skipped (--skip-envs) ==="
    echo ""
fi

# ============================================================================
# Step 4: Install R Packages (已废弃 - 使用 Go 替代方案)
# ============================================================================
if [ "$SKIP_R_PACKAGES" = false ]; then
    echo "=== Step 4: Skipped (R packages no longer required) ==="
    echo "  - matsrun 替代 RNA_Splicing.R"
    echo "  - seq2mat 替代 seq2mat.R"
    echo ""
else
    echo "=== Step 4: Skipped (--skip-r-packages) ==="
    echo ""
fi

# ============================================================================
# Summary
# ============================================================================
echo "======================================"
echo "Installation Summary"
echo "======================================"
echo ""
echo "Installation directory: $INSTALL_DIR"
echo "Runtime directory: $RUNTIME_DIR"
echo ""

# Check if binaries are in PATH
if [ -f "$INSTALL_DIR/otter" ]; then
    if command -v otter &> /dev/null; then
        echo "✓ otter is in PATH"
    else
        echo "⚠ Add to PATH: export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
fi

if [ -f "$INSTALL_DIR/enva" ]; then
    if command -v enva &> /dev/null; then
        echo "✓ enva is in PATH"
    else
        echo "⚠ Add to PATH: export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
fi


echo ""
echo "Next steps:"
echo "  1. Add binaries to PATH (if not already):"
echo "     export PATH=\"\$PATH:$INSTALL_DIR\""
echo ""
echo "  2. Verify installation:"
echo "     otter --version"
echo "     enva --version"
echo ""
echo "  3. Create a new analysis:"
echo "     cd $RUNTIME_DIR"
echo "     otter init my_project"
echo "     otter create --output my_project --fastq /path/to/fastq --pdata samples.csv \\"
echo "       --reference-root <registry> --reference-primary <id@release>"
echo "     otter config resolve --project my_project/project.yaml --backend local"
echo "     otter run --config my_project/runs/<run-id>/run.yaml --executor craftmake --phase step1 --dry-run"
echo ""
