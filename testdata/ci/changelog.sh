#!/bin/bash

echo "::set-output name=CHANGELOG::$(git log --pretty=format:'%s%n%b==============================================' --abbrev-commit $1..$2)"
