#!/bin/bash
# Gatewise Multi-Repo Distribution Script
# Run this after pushing the full code to a temporary repo

set -e

echo "🚀 Gatewise Multi-Repo Setup"
echo "================================"
echo ""

# Configuration
TEMP_REPO="votre-compte/gatewise-full"  # Le repo temporaire créé par Emergent
ORG="gatewise-io"

echo "📥 Step 1: Clone the full repository"
git clone https://github.com/$TEMP_REPO.git gatewise-full
cd gatewise-full

echo ""
echo "📦 Step 2: Prepare Community Edition"
# Clean Enterprise code
bash scripts/prepare-oss.sh

# Create Community repo
cd ..
mkdir gatewise-community
cp -r gatewise-full/* gatewise-community/
cd gatewise-community

# Remove Enterprise directories
rm -rf enterprise/
rm -rf internal/connector/kafka.go
rm -rf internal/connector/kong.go
rm -rf internal/jit/
rm -rf internal/reconciler/drift.go

# Use Community README
mv README-COMMUNITY.md README.md

# Use Community gitignore
mv .gitignore-community .gitignore

# Init git
git init
git add .
git commit -m "Initial commit - Gatewise Community Edition v0.3.0"
git branch -M main
git remote add origin https://github.com/$ORG/gatewise.git
git push -u origin main

echo "✅ Community repo pushed to: $ORG/gatewise"

echo ""
echo "🔒 Step 3: Prepare Enterprise Edition"
cd ../gatewise-full

# Keep only Enterprise code
mkdir -p gatewise-enterprise
cp -r enterprise/* gatewise-enterprise/
cp README-ENTERPRISE.md gatewise-enterprise/README.md

cd gatewise-enterprise
git init
git add .
git commit -m "Initial commit - Gatewise Enterprise v0.3.0"
git branch -M main
git remote add origin https://github.com/$ORG/gatewise-enterprise.git
git push -u origin main

echo "✅ Enterprise repo pushed to: $ORG/gatewise-enterprise"

echo ""
echo "📊 Step 4: Prepare Helm Charts"
cd ../gatewise-full
mkdir gatewise-helm
cp -r helm/gatewise/* gatewise-helm/

cd gatewise-helm
cat > README.md << 'EOF'
# Gatewise Helm Charts

Official Helm charts for Gatewise.

## Installation

```bash
helm repo add gatewise https://gatewise-io.github.io/gatewise-helm
helm install gatewise gatewise/gatewise
```

## Charts

- `gatewise` - Main Gatewise installation
- `gatewise-enterprise` - Enterprise edition

## Documentation

See [docs.gatewise.io](https://docs.gatewise.io) for full documentation.
EOF

git init
git add .
git commit -m "Initial commit - Gatewise Helm Charts"
git branch -M main
git remote add origin https://github.com/$ORG/gatewise-helm.git
git push -u origin main

echo "✅ Helm repo pushed to: $ORG/gatewise-helm"

echo ""
echo "🎉 Setup Complete!"
echo ""
echo "📋 Summary:"
echo "  ✅ Community: https://github.com/$ORG/gatewise"
echo "  ✅ Enterprise: https://github.com/$ORG/gatewise-enterprise"
echo "  ✅ Helm: https://github.com/$ORG/gatewise-helm"
echo ""
echo "🔖 Next steps:"
echo "  1. Create a release: cd gatewise-community && git tag v0.3.0 && git push --tags"
echo "  2. GitHub Actions will automatically build and release binaries"
echo "  3. Setup GitHub Pages for Helm repo"
