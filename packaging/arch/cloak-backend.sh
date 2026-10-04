#!/bin/sh
# Entry point for the packaged cloak-backend venv (seanime-cloak-backend-git).
exec /opt/seanime-cloak-backend/bin/python -m cloak_backend "$@"
