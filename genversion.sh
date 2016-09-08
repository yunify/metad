VERSION=${TRAVIS_TAG:-1.0.dev}
GIT_VERSION=$(git rev-parse --verify -q HEAD)

cat <<EOT > version.go
package main

const VERSION = "$VERSION"
const GIT_VERSION = "$GIT_VERSION"
EOT
