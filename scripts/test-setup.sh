#!/bin/bash
# Quick test script for tomeko19

echo "Testing Gatewise multi-repo setup..."
echo ""
echo "Your current repo: https://github.com/tomeko19/gatewise"
echo ""
echo "This will create:"
echo "  - gatewise-io/gatewise (Community - Public)"
echo "  - gatewise-io/gatewise-enterprise (Private)"
echo "  - gatewise-io/gatewise-helm (Public)"
echo ""
echo "Prerequisites check:"
echo ""

# Check if git is installed
if command -v git &> /dev/null; then
    echo "✅ Git installed: $(git --version)"
else
    echo "❌ Git not found. Install it first!"
    exit 1
fi

# Check git config
if git config user.email &> /dev/null; then
    echo "✅ Git configured: $(git config user.email)"
else
    echo "⚠️  Git email not configured. Run:"
    echo "   git config --global user.email 'your@email.com'"
fi

# Check if organization exists (this will fail if it doesn't, which is expected)
echo ""
echo "⚠️  Make sure you have created the GitHub organization 'gatewise-io'"
echo "   Create it at: https://github.com/organizations/plan"
echo ""
echo "⚠️  And create these 3 empty repos:"
echo "   1. gatewise-io/gatewise (Public)"
echo "   2. gatewise-io/gatewise-enterprise (Private)"
echo "   3. gatewise-io/gatewise-helm (Public)"
echo ""
read -p "Press ENTER when repos are created, or CTRL+C to cancel..."

echo ""
echo "🚀 Ready to run setup-multi-repos.sh"
echo ""
echo "Run: bash /app/scripts/setup-multi-repos.sh"
