#!/bin/bash

echo "::set-output name=CHANGELOG::$(git log --pretty=format:'%s%n%b==============================================' --abbrev-commit $1..$2)"

if git describe --tags --exact-match; then
    echo "::set-output name=VERSION::$(git describe --tags --exact-match | sed 's/^v//g')"
else
    echo "::set-output name=VERSION::$(git describe --long --tags | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g')"
fi
