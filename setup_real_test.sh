#!/bin/bash

# Setup script for real API testing

print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}\033[0m"
}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'

print_status $CYAN "🔧 Autocache Real Testing Setup"
print_status $CYAN "==============================="

# Check if .env exists and has real API key
if [ -f ".env" ]; then
    API_KEY=$(grep "ANTHROPIC_API_KEY=" .env | cut -d'=' -f2)
    if [ "$API_KEY" = "test-key" ] || [ -z "$API_KEY" ]; then
        print_status $YELLOW "⚠️  .env file exists but needs a real API key"
        NEEDS_API_KEY=true
    else
        print_status $GREEN "✅ API key found in .env"
        NEEDS_API_KEY=false
    fi
else
    print_status $YELLOW "⚠️  No .env file found"
    NEEDS_API_KEY=true
fi

if [ "$NEEDS_API_KEY" = true ]; then
    print_status $BLUE "📝 Setting up .env file..."

    echo "Please enter your Anthropic API key (starts with sk-ant-api03-):"
    read -r USER_API_KEY

    if [[ $USER_API_KEY == sk-ant-api03-* ]]; then
        # Create or update .env file
        cat > .env << EOF
# Autocache Configuration for Real Testing
ANTHROPIC_API_KEY=$USER_API_KEY
PORT=8080
HOST=0.0.0.0
ANTHROPIC_API_URL=https://api.anthropic.com
CACHE_STRATEGY=moderate
LOG_LEVEL=info
LOG_JSON=false
ENABLE_METRICS=true
ENABLE_DETAILED_ROI=true
MAX_CACHE_BREAKPOINTS=4
TOKEN_MULTIPLIER=1.0
EOF
        print_status $GREEN "✅ .env file created with your API key"
    else
        print_status $RED "❌ Invalid API key format. Should start with 'sk-ant-api03-'"
        exit 1
    fi
fi

# Check if binary exists
if [ ! -f "autocache" ]; then
    print_status $BLUE "🔨 Building autocache..."
    if go build -o autocache; then
        print_status $GREEN "✅ Build successful"
    else
        print_status $RED "❌ Build failed"
        exit 1
    fi
else
    print_status $GREEN "✅ autocache binary found"
fi

# Make test script executable
chmod +x test_real.sh

print_status $CYAN "\n🚀 Setup Complete! Ready for real testing."
print_status $BLUE "Next steps:"
echo "  1. Start the proxy: ./autocache"
echo "  2. In another terminal: ./test_real.sh"
echo ""
print_status $YELLOW "💡 Tips:"
echo "  - Monitor proxy logs to see cache decisions"
echo "  - Check test_results/ folder for detailed analysis"
echo "  - Try different CACHE_STRATEGY values (conservative/moderate/aggressive)"