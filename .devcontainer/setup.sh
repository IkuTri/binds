#!/bin/bash
set -e

echo "🔧 Building bd from source..."
go build -o bd ./cmd/bd

echo "📦 Installing bd globally..."
sudo mv bd /usr/local/bin/bd
sudo chmod +x /usr/local/bin/bd

echo "✅ Verifying bd installation..."
bd version

echo "🎯 Initializing bd (non-interactive)..."
if [ ! -f .binds/beads.db ]; then
  bd init --quiet
else
  echo "bd already initialized"
fi

echo "🪝 Installing git hooks..."
if [ -f examples/git-hooks/install.sh ]; then
  bash examples/git-hooks/install.sh
  echo "Git hooks installed successfully"
else
  echo "⚠️  Git hooks installer not found, skipping..."
fi

echo "📚 Installing Go dependencies..."
go mod download

echo "✨ Development environment ready!"
echo "Run 'bd ready' to see available tasks"
