#!/bin/bash

container_name="soa-mafia_client_$1"
docker logs "$container_name" && docker attach "$container_name"