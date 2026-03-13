#!/bin/bash
# Gatewise Multi-Repo Distribution Script
# Configured for: tomeko19/gatewise

set -e

echo "🚀 Gatewise Multi-Repo Setup for tomeko19"
echo "=========================================="
echo ""

# Your configuration
SOURCE_REPO="tomeko19/gatewise"
ORG="gatewise-io"

echo "ℹ️  Configuration:"
echo "  Source: https://github.com/$SOURCE_REPO"
echo "  Target Organization: $ORG"
echo ""
echo "📋 Prerequisites:"
echo "  1. GitHub org '$ORG' must exist"
echo "  2. Create these empty repos in the org:"
echo "     - $ORG/gatewise (Public, MIT License)"
echo "     - $ORG/gatewise-enterprise (Private)"
echo "     - $ORG/gatewise-helm (Public, MIT License)"
echo ""
read -p "Press ENTER when ready..."

echo ""
echo "📥 Step 1: Clone your repository"
if [ -d "gatewise-source" ]; then
    echo "  Directory exists, pulling latest..."
    cd gatewise-source
    git pull
    cd ..
else
    git clone https://github.com/$SOURCE_REPO.git gatewise-source
fi

cd gatewise-source

echo ""
echo "🧹 Step 2: Prepare Community Edition"
echo "  Cleaning Enterprise code..."

# Create Community directory
cd ..
rm -rf gatewise-community
cp -r gatewise-source gatewise-community
cd gatewise-community

# Remove Enterprise code
echo "  Removing Enterprise features..."
rm -rf gatewise/enterprise/ 2>/dev/null || true
rm -f gatewise/internal/connector/kafka.go 2>/dev/null || true
rm -f gatewise/internal/connector/kong.go 2>/dev/null || true
rm -rf gatewise/internal/jit/ 2>/dev/null || true
rm -f gatewise/internal/reconciler/drift.go 2>/dev/null || true

# Use Community README
if [ -f "README-COMMUNITY.md" ]; then
    mv README-COMMUNITY.md README.md
fi

# Use Community gitignore
if [ -f ".gitignore-community" ]; then
    cp .gitignore-community .gitignore
fi

# Remove Enterprise-specific files
rm -f README-ENTERPRISE.md
rm -f ENTERPRISE-SEPARATION.md

# Update git remote
git remote remove origin 2>/dev/null || true
git remote add origin https://github.com/$ORG/gatewise.git

echo ""
echo "📤 Pushing Community Edition..."
git add .
git commit -m "feat: Gatewise Community Edition v0.3.0

- Kubernetes orchestration
- Policy as Code
- GitOps Operator with CRD
- Prometheus metrics
- CLI & REST API
- Dashboard UI

Enterprise features (Kafka, Kong, JIT, Self-Healing) require separate license.
" || echo "No changes to commit"

git push -u origin main --force

echo "✅ Community repo: https://github.com/$ORG/gatewise"

echo ""
echo "🔒 Step 3: Prepare Enterprise Edition"
cd ../gatewise-source

# Create Enterprise directory structure
rm -rf ../gatewise-enterprise
mkdir -p ../gatewise-enterprise

# Copy Enterprise code
if [ -d "gatewise/enterprise" ]; then
    cp -r gatewise/enterprise/* ../gatewise-enterprise/
fi

# Add Enterprise README
cat > ../gatewise-enterprise/README.md << 'EOF'
# Gatewise Enterprise

**Private Repository - Proprietary Code**

Enterprise features for Gatewise including:
- Kafka connector (Topics, ACLs)
- Kong connector (Routes, Services)
- JIT Access with approval workflows
- Self-Healing (drift detection)
- Multi-cluster management

## License

Proprietary. All rights reserved.

For licensing: sales@gatewise.io
EOF

cd ../gatewise-enterprise
git init
git add .
git commit -m "feat: Gatewise Enterprise Edition v0.3.0

Enterprise-only features:
- Kafka connector
- Kong connector  
- JIT Access manager
- Self-Healing drift detection
"

git remote add origin https://github.com/$ORG/gatewise-enterprise.git
git push -u origin main --force

echo "✅ Enterprise repo: https://github.com/$ORG/gatewise-enterprise"

echo ""
echo "📊 Step 4: Prepare Helm Charts"
cd ../gatewise-source

rm -rf ../gatewise-helm
mkdir -p ../gatewise-helm

# Copy Helm charts
if [ -d "gatewise/helm/gatewise" ]; then
    cp -r gatewise/helm/gatewise/* ../gatewise-helm/
fi

# Add Helm README
cat > ../gatewise-helm/README.md << 'EOF'
# Gatewise Helm Charts

Official Helm charts for deploying Gatewise on Kubernetes.

## Usage

```bash
# Add the Gatewise Helm repository
helm repo add gatewise https://gatewise-io.github.io/gatewise-helm
helm repo update

# Install Gatewise Community Edition
helm install gatewise gatewise/gatewise

# Install with custom values
helm install gatewise gatewise/gatewise -f values.yaml
```

## Charts

- **gatewise** - Community Edition (Kubernetes orchestration)
- **gatewise-enterprise** - Enterprise Edition (requires license)

## Configuration

See [values.yaml](values.yaml) for all configuration options.

## Documentation

Full documentation: https://docs.gatewise.io

## Support

- Community: [GitHub Discussions](https://github.com/gatewise-io/gatewise/discussions)
- Enterprise: support@gatewise.io
EOF

cd ../gatewise-helm
git init
git add .
git commit -m "feat: Gatewise Helm Charts v0.3.0

Official Helm charts for Gatewise deployment.
"

git remote add origin https://github.com/$ORG/gatewise-helm.git
git push -u origin main --force

echo "✅ Helm repo: https://github.com/$ORG/gatewise-helm"

echo ""
echo "🎉 Setup Complete!"
echo ""
echo "📋 Your repositories:"
echo "  ✅ Source: https://github.com/$SOURCE_REPO"
echo "  ✅ Community: https://github.com/$ORG/gatewise"
echo "  ✅ Enterprise: https://github.com/$ORG/gatewise-enterprise"
echo "  ✅ Helm: https://github.com/$ORG/gatewise-helm"
echo ""
echo "🔖 Next steps:"
echo ""
echo "1. Create a release:"
echo "   cd ../gatewise-community"
echo "   git tag v0.3.0"
echo "   git push origin v0.3.0"
echo ""
echo "2. GitHub Actions will automatically:"
echo "   - Build binaries for all platforms"
echo "   - Create release with download links"
echo "   - Build Docker images"
echo ""
echo "3. Setup GitHub Pages for Helm (optional):"
echo "   - Go to $ORG/gatewise-helm"
echo "   - Settings → Pages → Deploy from main branch"
echo ""
echo "4. Keep your source repo private:"
echo "   - Go to https://github.com/$SOURCE_REPO/settings"
echo "   - Change visibility to Private"
echo "   - Or delete it if you don't need it anymore"
