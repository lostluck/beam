#!/bin/bash
#
#    Licensed to the Apache Software Foundation (ASF) under one or more
#    contributor license agreements.  See the NOTICE file distributed with
#    this work for additional information regarding copyright ownership.
#    The ASF licenses this file to You under the Apache License, Version 2.0
#    (the "License"); you may not use this file except in compliance with
#    the License.  You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.

# This script sets the go version used by all Beam SDK scripts.
# It requires an existing Go installation on the system, which
# will be used to download specific versions of Go.
#
# Accepts the following optional flags, after which all following parameters
# will be provided to the go version tool.
#    --version -> A string for a fully qualified go version, eg go1.16.5 or go1.18beta1
#        The list of available versions are at https://go.dev/dl/ 


set -e

# The specific Go version used by default for Beam infrastructure.
#
# This variable is also used as the execution command downscript.
# The list of downloadable versions are at https://go.dev/dl/ 
GOVERS=go1.16.12

if ! command -v go &> /dev/null
then
    echo "go could not be found. This script requires a go installation to boostrap using specific go versions. "
    exit 1
fi

while [[ $# -gt 0 ]]
do
key="$1"
case $key in
    --version)
        GOVERS="$2"
        shift # past argument
        shift # past value
        ;;
    *)  # unknown options are go tool args.
        break
        ;;
esac
done

# Outputing the system Go version for debugging purposes.
echo "System Go installation: `which go` is `go version`"
echo "Preparing to use $GOBIN/$GOVERS"


GOPATH=`go env GOPATH`
GOBIN=$GOPATH/bin

echo GOPATH=$GOPATH
echo GOBIN=$GOBIN

GOBIN=$GOBIN go install golang.org/dl/$GOVERS@latest

# This operation is cached on system and won't be re-downloaded.
$GOBIN/$GOVERS download

$GOBIN/$GOVERS $@
