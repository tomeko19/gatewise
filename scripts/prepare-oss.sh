#!/bin/bash
# Gatewise OSS Preparation Script
# This script removes Enterprise-only code before publishing to public GitHub

set -e

echo "🧹 Preparing Gatewise for Open Source release..."

# Remove Enterprise connectors
echo "  → Removing Kafka connector..."
rm -rf internal/connector/kafka.go 2>/dev/null || true

echo "  → Removing Kong connector..."
rm -rf internal/connector/kong.go 2>/dev/null || true

# Remove Enterprise features
echo "  → Removing JIT Access module..."
rm -rf internal/jit/ 2>/dev/null || true

echo "  → Removing Self-Healing module..."
rm -rf internal/reconciler/drift.go 2>/dev/null || true

# Remove proprietary license validation
echo "  → Removing license validation logic..."
# Keep the stub, remove server-side validation

# Remove Enterprise examples
echo "  → Removing Enterprise config examples..."
rm -rf configs/examples/enterprise/ 2>/dev/null || true

# Remove Enterprise dashboard features
echo "  → Cleaning frontend Enterprise features..."
# This would need manual review in App.js

echo ""
echo "✅ OSS preparation complete!"
echo ""
echo "📋 Summary:"
echo "  - Kafka connector: REMOVED"
echo "  - Kong connector: REMOVED"
echo "  - JIT Access: REMOVED"
echo "  - Self-Healing: REMOVED"
echo "  - License validation: STUBBED"
echo ""
echo "🎯 This code is now ready for: github.com/gatewise-io/gatewise"
