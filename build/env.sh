#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
essdir="$workspace/src/github.com/orangeAndSuns"
if [ ! -L "$essdir/essentia" ]; then
    mkdir -p "$essdir"
    cd "$essdir"
    ln -s ../../../../../. essentia
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$essdir/essentia"
PWD="$essdir/essentia"

# Launch the arguments with the configured environment.
exec "$@"
