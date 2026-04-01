#!/bin/bash
#
# IntelLegal Installation Script
# Supports: WSL2 Ubuntu, macOS, Ubuntu Linux
#
# This script will:
# 1. Detect the operating system
# 2. Install all required dependencies
# 3. Set up Docker and Docker Compose
# 4. Configure the environment
# 5. Build and start all services
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_not_root() {
    if [[ $EUID -eq 0 ]]; then
        log_error "This script should not be run as root"
        exit 1
    fi
}

# Detect operating system
detect_os() {
    if [[ "$(uname)" == "Darwin" ]]; then
        OS="macos"
    elif [[ -f /etc/os-release ]]; then
        . /etc/os-release
        if [[ "$ID" == "ubuntu" ]]; then
            OS="ubuntu"
        elif [[ "$ID" == "debian" ]]; then
            OS="debian"
        else
            OS="linux"
        fi
    elif [[ "$(uname -r | grep -i microsoft)" ]]; then
        OS="wsl2"
    else
        OS="unknown"
    fi
    
    log_info "Detected operating system: $OS"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if running in WSL2
is_wsl2() {
    if grep -qi microsoft /proc/version 2>/dev/null; then
        return 0
    fi
    return 1
}

# Install dependencies for WSL2 Ubuntu
install_wsl2() {
    log_info "Installing dependencies for WSL2 Ubuntu..."
    
    # Update package list
    log_info "Updating package list..."
    sudo apt update
    
    # Install system dependencies
    log_info "Installing system dependencies..."
    sudo apt install -y \
        git \
        make \
        curl \
        gnupg \
        software-properties-common \
        apt-transport-https \
        ca-certificates \
        lsb-release
    
    # Install Python 3.14 (or latest available)
    log_info "Installing Python..."
    if ! command_exists python3; then
        sudo add-apt-repository -y ppa:deadsnakes/ppa 2>/dev/null || true
        sudo apt update
        sudo apt install -y python3 python3-venv python3-pip || {
            log_warning "Python 3.14 not available, installing default python3"
            sudo apt install -y python3 python3-venv python3-pip
        }
    fi
    
    # Install Go
    if ! command_exists go; then
        log_info "Installing Go..."
        GO_VERSION="1.25.0"
        wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        if ! grep -q "/usr/local/go/bin" ~/.bashrc 2>/dev/null; then
            echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        fi
        export PATH=$PATH:/usr/local/go/bin
    fi
    
    # Install Node.js
    if ! command_exists node; then
        log_info "Installing Node.js..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt install -y nodejs
    fi
    
    # Install Docker
    if ! command_exists docker; then
        log_info "Installing Docker..."
        # Remove old versions
        sudo apt remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true
        
        # Add Docker's official GPG key
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
        
        # Set up the repository
        echo \
            "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
            $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        
        # Install Docker
        sudo apt update
        sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
    fi
    
    # Add user to docker group
    if ! groups $USER | grep -q docker; then
        log_info "Adding user to docker group..."
        sudo usermod -aG docker $USER
        log_warning "You need to log out and log back in for docker group changes to take effect"
    fi
    
    log_success "WSL2 dependencies installed"
}

# Install dependencies for Ubuntu Linux
install_ubuntu() {
    log_info "Installing dependencies for Ubuntu Linux..."
    
    # Update package list
    log_info "Updating package list..."
    sudo apt update
    
    # Install system dependencies
    log_info "Installing system dependencies..."
    sudo apt install -y \
        git \
        make \
        curl \
        gnupg \
        software-properties-common \
        apt-transport-https \
        ca-certificates \
        lsb-release
    
    # Install Python 3.14 (or latest available)
    log_info "Installing Python..."
    if ! command_exists python3; then
        sudo add-apt-repository -y ppa:deadsnakes/ppa 2>/dev/null || true
        sudo apt update
        sudo apt install -y python3 python3-venv python3-pip || {
            log_warning "Python 3.14 not available, installing default python3"
            sudo apt install -y python3 python3-venv python3-pip
        }
    fi
    
    # Install Go
    if ! command_exists go; then
        log_info "Installing Go..."
        GO_VERSION="1.25.0"
        wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        if ! grep -q "/usr/local/go/bin" ~/.bashrc 2>/dev/null; then
            echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        fi
        export PATH=$PATH:/usr/local/go/bin
    fi
    
    # Install Node.js
    if ! command_exists node; then
        log_info "Installing Node.js..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt install -y nodejs
    fi
    
    # Install Docker
    if ! command_exists docker; then
        log_info "Installing Docker..."
        # Remove old versions
        sudo apt remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true
        
        # Add Docker's official GPG key
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
        
        # Set up the repository
        echo \
            "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
            $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        
        # Install Docker
        sudo apt update
        sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
    fi
    
    # Add user to docker group
    if ! groups $USER | grep -q docker; then
        log_info "Adding user to docker group..."
        sudo usermod -aG docker $USER
        log_warning "You need to log out and log back in for docker group changes to take effect"
    fi
    
    log_success "Ubuntu dependencies installed"
}

# Install dependencies for macOS
install_macos() {
    log_info "Installing dependencies for macOS..."
    
    # Install Homebrew if not installed
    if ! command_exists brew; then
        log_info "Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        
        # Add Homebrew to PATH
        if [[ "$(uname -m)" == "arm64" ]]; then
            BREW_PATH="/opt/homebrew/bin"
        else
            BREW_PATH="/usr/local/bin"
        fi
        
        if ! grep -q "$BREW_PATH" ~/.zprofile 2>/dev/null; then
            echo "eval \"\$($BREW_PATH/brew shellenv)\"" >> ~/.zprofile
        fi
        eval "$($BREW_PATH/brew shellenv)"
    fi
    
    # Install dependencies
    log_info "Installing dependencies with Homebrew..."
    brew update
    
    # Install git
    if ! command_exists git; then
        brew install git
    fi
    
    # Install make (GNU make)
    if ! command_exists gmake; then
        brew install make
    fi
    
    # Install Python
    if ! command_exists python3; then
        brew install python
    fi
    
    # Install Go
    if ! command_exists go; then
        brew install go
    fi
    
    # Install Node.js
    if ! command_exists node; then
        brew install node
    fi
    
    # Install Docker CLI (Docker Desktop provides the daemon)
    if ! command_exists docker; then
        log_info "Installing Docker CLI..."
        brew install --cask docker
        log_warning "Please start Docker Desktop application to complete Docker setup"
    fi
    
    log_success "macOS dependencies installed"
}

# Verify installations
verify_installations() {
    log_info "Verifying installations..."
    
    local errors=0
    
    # Check Docker
    if command_exists docker; then
        DOCKER_VERSION=$(docker --version 2>&1)
        log_info "Docker: $DOCKER_VERSION"
    else
        log_error "Docker is not installed"
        errors=$((errors + 1))
    fi
    
    # Check Docker Compose
    if command_exists docker compose; then
        COMPOSE_VERSION=$(docker compose version 2>&1)
        log_info "Docker Compose: $COMPOSE_VERSION"
    else
        log_error "Docker Compose is not installed"
        errors=$((errors + 1))
    fi
    
    # Check Go
    if command_exists go; then
        GO_VERSION=$(go version 2>&1)
        log_info "Go: $GO_VERSION"
    else
        log_error "Go is not installed"
        errors=$((errors + 1))
    fi
    
    # Check Python
    if command_exists python3; then
        PYTHON_VERSION=$(python3 --version 2>&1)
        log_info "Python: $PYTHON_VERSION"
    else
        log_error "Python is not installed"
        errors=$((errors + 1))
    fi
    
    # Check Node.js
    if command_exists node; then
        NODE_VERSION=$(node --version 2>&1)
        log_info "Node.js: $NODE_VERSION"
    else
        log_error "Node.js is not installed"
        errors=$((errors + 1))
    fi
    
    # Check npm
    if command_exists npm; then
        NPM_VERSION=$(npm --version 2>&1)
        log_info "npm: $NPM_VERSION"
    fi
    
    # Check Make
    if command_exists make || command_exists gmake; then
        if command_exists gmake; then
            MAKE_VERSION=$(gmake --version 2>&1 | head -n 1)
        else
            MAKE_VERSION=$(make --version 2>&1 | head -n 1)
        fi
        log_info "Make: $MAKE_VERSION"
    else
        log_error "Make is not installed"
        errors=$((errors + 1))
    fi
    
    # Check Git
    if command_exists git; then
        GIT_VERSION=$(git --version 2>&1)
        log_info "Git: $GIT_VERSION"
    else
        log_error "Git is not installed"
        errors=$((errors + 1))
    fi
    
    if [[ $errors -gt 0 ]]; then
        log_error "$errors verification error(s) found"
        return 1
    fi
    
    log_success "All installations verified successfully"
    return 0
}

# Check if a port is available
check_port_available() {
    local port=$1
    local host=${2:-"localhost"}
    
    # Check if port is in use using lsof or netstat
    if command_exists lsof; then
        if lsof -i :$port >/dev/null 2>&1; then
            return 1  # Port is in use
        fi
    elif command_exists netstat; then
        if netstat -an | grep -q ":$port "; then
            return 1  # Port is in use
        fi
    elif command_exists ss; then
        if ss -tuln | grep -q ":$port "; then
            return 1  # Port is in use
        fi
    else
        # Fallback: try to bind to the port using bash
        if (echo >/dev/tcp/$host/$port) 2>/dev/null; then
            return 1  # Port is in use
        fi
    fi
    
    return 0  # Port is available
}

# Find an available port starting from a base port
find_available_port() {
    local base_port=$1
    local max_attempts=${2:-100}
    local port=$base_port
    
    for ((i=0; i<max_attempts; i++)); do
        if check_port_available $port; then
            echo $port
            return 0
        fi
        port=$((port + 1))
    done
    
    echo $base_port  # Return original if no available port found
    return 1
}

# Get current port value from .env file
get_env_port() {
    local var_name=$1
    local env_file=$2
    local value
    
    if [[ -f "$env_file" ]]; then
        value=$(grep "^${var_name}=" "$env_file" 2>/dev/null | cut -d'=' -f2 | tr -d '[:space:]')
    fi
    
    echo "${value:-0}"
}

# Update port value in .env file
update_env_port() {
    local var_name=$1
    local new_port=$2
    local env_file=$3
    
    if [[ -f "$env_file" ]]; then
        if grep -q "^${var_name}=" "$env_file" 2>/dev/null; then
            sed -i.bak "s/^${var_name}=.*/${var_name}=${new_port}/" "$env_file"
            rm -f "${env_file}.bak"
        fi
    fi
}

# Check and adjust ports in .env file
check_and_adjust_ports() {
    local env_file=$1
    local ports_adjusted=0
    
    log_info "Checking port availability..."
    
    # Define ports to check using individual variables
    local frontend_port=3000
    local go_api_port=8080
    local py_ai_api_port=8000
    local postgres_port=5432
    local qdrant_http_port=6333
    local qdrant_grpc_port=6334
    local redis_port=6379
    
    # Array of port variables and their names
    local port_configs=(
        "FRONTEND_PORT:$frontend_port"
        "GO_API_PORT:$go_api_port"
        "PY_AI_API_PORT:$py_ai_api_port"
        "POSTGRES_PORT:$postgres_port"
        "QDRANT_HTTP_PORT:$qdrant_http_port"
        "QDRANT_GRPC_PORT:$qdrant_grpc_port"
        "REDIS_PORT:$redis_port"
    )
    
    # Track used ports to avoid conflicts (using string for bash 3.2 compatibility)
    local used_ports=""
    
    for port_config in "${port_configs[@]}"; do
        local var_name="${port_config%%:*}"
        local default_port="${port_config#*:}"
        local current_port=$(get_env_port "$var_name" "$env_file")
        
        # Use current port from .env if set, otherwise use default
        if [[ -z "$current_port" || "$current_port" == "0" ]]; then
            current_port=$default_port
        fi
        
        # Check if port is in our used_ports list
        port_in_use() {
            local port=$1
            [[ " $used_ports " == *" $port "* ]]
        }
        
        # Check if current port is available and not already used by another service
        if ! check_port_available "$current_port" || port_in_use "$current_port"; then
            # Find an available port that's not already taken by another service
            local available_port=$(find_available_port "$current_port")
            
            # Make sure we don't pick a port that's already been assigned to another service
            while port_in_use "$available_port"; do
                available_port=$((available_port + 1))
            done
            
            if [[ $available_port -ne $current_port ]]; then
                log_info "Port $current_port is in use, using $available_port instead"
                update_env_port "$var_name" "$available_port" "$env_file"
                used_ports="$used_ports $available_port"
                ports_adjusted=$((ports_adjusted + 1))
            else
                log_warning "Could not find available port near $current_port"
            fi
        else
            # Mark this port as used
            used_ports="$used_ports $current_port"
        fi
    done
    
    if [[ $ports_adjusted -gt 0 ]]; then
        log_info "Adjusted $ports_adjusted port(s) in $env_file"
    else
        log_success "All ports are available"
    fi
}

# Setup environment files
setup_environment() {
    log_info "Setting up environment files..."
    
    # Setup root .env file
    if [[ ! -f .env ]]; then
        log_info "Creating .env file from .env.example..."
        cp .env.example .env
        log_success "Created .env file"
    else
        log_info ".env file already exists"
    fi
    
    # Check and adjust ports if needed
    check_and_adjust_ports ".env"
    
    # Setup frontend .env file
    if [[ ! -f frontend/.env ]]; then
        log_info "Creating frontend/.env file from frontend/.env.example..."
        cp frontend/.env.example frontend/.env
        log_success "Created frontend/.env file"
    else
        log_info "frontend/.env file already exists"
    fi
    
    # Generate secrets if not set
    if ! grep -q "^JWT_SECRET=" .env 2>/dev/null || [[ $(grep "^JWT_SECRET=" .env | cut -d'=' -f2) == "" ]]; then
        log_info "Generating JWT secret..."
        local jwt_secret=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p)
        if grep -q "^JWT_SECRET=" .env 2>/dev/null; then
            sed -i.bak "s/^JWT_SECRET=.*/JWT_SECRET=${jwt_secret}/" .env
        else
            echo "JWT_SECRET=${jwt_secret}" >> .env
        fi
        rm -f .env.bak 2>/dev/null
    fi
    
    if ! grep -q "^MINIO_ROOT_PASSWORD=" .env 2>/dev/null || [[ $(grep "^MINIO_ROOT_PASSWORD=" .env | cut -d'=' -f2) == "" ]]; then
        log_info "Generating MinIO root password..."
        local minio_password=$(openssl rand -base64 24 2>/dev/null || head -c 24 /dev/urandom | base64)
        if grep -q "^MINIO_ROOT_PASSWORD=" .env 2>/dev/null; then
            sed -i.bak "s/^MINIO_ROOT_PASSWORD=.*/MINIO_ROOT_PASSWORD=${minio_password}/" .env
        else
            echo "MINIO_ROOT_PASSWORD=${minio_password}" >> .env
        fi
        rm -f .env.bak 2>/dev/null
    fi
    
    log_success "Environment setup complete"
}

# Build and start all services
build_and_start() {
    log_info "Building and starting all services..."
    
    # Ensure Docker is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    
    # Pull and build all services
    log_info "Building Docker containers..."
    docker compose build --pull
    
    # Start all services
    log_info "Starting services..."
    docker compose up -d
    
    # Wait for services to be ready
    log_info "Waiting for services to start..."
    sleep 10
    
    check_health
    
    log_success "All services started successfully"
}

# Check health of all services
check_health() {
    log_info "Checking service health..."
    
    # Check PostgreSQL
    if docker compose ps postgres 2>/dev/null | grep -q "Up"; then
        log_success "PostgreSQL is running"
    else
        log_warning "PostgreSQL may not be ready yet"
    fi
    
    # Check Qdrant
    if docker compose ps qdrant 2>/dev/null | grep -q "Up"; then
        log_success "Qdrant is running"
    else
        log_warning "Qdrant may not be ready yet"
    fi
    
    # Check Redis
    if docker compose ps redis 2>/dev/null | grep -q "Up"; then
        log_success "Redis is running"
    else
        log_warning "Redis may not be ready yet"
    fi
    
    # Check MinIO
    if docker compose ps minio 2>/dev/null | grep -q "Up"; then
        log_success "MinIO is running"
    else
        log_warning "MinIO may not be ready yet"
    fi
    
    # Check Go API
    if docker compose ps go-api 2>/dev/null | grep -q "Up"; then
        log_success "Go API is running"
    else
        log_warning "Go API may not be ready yet"
    fi
    
    # Check Python AI API
    if docker compose ps py-ai-api 2>/dev/null | grep -q "Up"; then
        log_success "Python AI API is running"
    else
        log_warning "Python AI API may not be ready yet"
    fi
    
    # Check Frontend
    if docker compose ps frontend 2>/dev/null | grep -q "Up"; then
        log_success "Frontend is running"
    else
        log_warning "Frontend may not be ready yet"
    fi
}

# Print completion message
print_completion_message() {
    echo ""
    log_success "=========================================="
    log_success "   IntelLegal Installation Complete!"
    log_success "=========================================="
    echo ""
    log_info "Access the application at:"
    echo ""
    echo "   Frontend:    http://localhost:3000"
    echo "   Go API:      http://localhost:8080"
    echo "   Python AI:   http://localhost:8000"
    echo ""
    log_info "Useful commands:"
    echo ""
    echo "   View logs:       docker compose logs -f"
    echo "   Stop services:   docker compose down"
    echo "   Restart:         docker compose restart"
    echo "   Check status:    docker compose ps"
    echo ""
    log_info "For more information, see docs/INSTALLATION.md"
    echo ""
}

# Main function
main() {
    echo ""
    log_info "=========================================="
    log_info "   IntelLegal Installation Script"
    log_info "=========================================="
    echo ""
    
    check_not_root
    detect_os
    
    case $OS in
        wsl2)
            install_wsl2
            ;;
        ubuntu|debian)
            install_ubuntu
            ;;
        macos)
            install_macos
            ;;
        *)
            log_error "Unsupported operating system: $OS"
            log_error "This script supports WSL2 Ubuntu, macOS, and Ubuntu Linux"
            exit 1
            ;;
    esac
    
    verify_installations
    
    setup_environment
    
    build_and_start
    
    print_completion_message
}

# Run main function
main "$@"
