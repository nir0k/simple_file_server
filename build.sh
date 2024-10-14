#!/bin/bash

set -e

docker build -t file_server_debian10 .
# Create a temporary container
docker create --name temp_container file_server_debian10

# Copy the compiled binary from the container to the host machine
docker cp temp_container:/app/bin/file_server ./file_server

# Remove the temporary container
docker rm temp_container
