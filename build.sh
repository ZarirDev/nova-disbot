#!/usr/bin/env bash

set -Eeuo pipefail

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
RESET='\033[0m'

step=0
total=4

progress() {
    step=$((step + 1))
    printf "\n${BLUE}[%d%%] %s${RESET}\n" \
        "$((step * 100 / total))" "$1"
}

success() {
    printf "${GREEN}✔ %s${RESET}\n" "$1"
}

fail() {
    printf "${RED}✘ Build failed at line %s${RESET}\n" "$1"
}

trap 'fail "$LINENO"' ERR

printf "${BLUE}====================================${RESET}\n"
printf "${BLUE}       NOVA-DISBOT BUILD             ${RESET}\n"
printf "${BLUE}====================================${RESET}\n"

progress "Preparing build directory..."
mkdir -p ./build
success "Build directory ready"

progress "Compiling Go application..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o ./build/nova-disbot .
# go build -v -o ./build/nova-disbot main.go
success "Compilation complete"

progress "Copying environment template..."
cp ./.env.example ./build/.env
success "Environment template copied"

progress "Checking build artifact..."
test -x ./build/nova-disbot
test -f ./build/.env
success "Build artifacts verified"

printf "\n${GREEN}====================================${RESET}\n"
printf "${GREEN} BUILD COMPLETED SUCCESSFULLY (100%%)${RESET}\n"
printf "${GREEN} Binary: ./build/nova-disbot${RESET}\n"
printf "${GREEN} Config: ./build/.env${RESET}\n"
printf "${GREEN} MAKE SURE TO EDIT THE .env FILE${RESET}\n"
printf "${GREEN}====================================${RESET}\n"