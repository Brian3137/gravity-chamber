#!/bin/sh
# initdb writes a pg_hba.conf that permits replication connections from
# localhost only. A standby in another container is not localhost, so
# pg_basebackup gets "no pg_hba.conf entry for replication connection".
#
# `all` for the source address is fine on a compose network you control and
# nowhere else — in production this is a specific CIDR and a dedicated role
# with the REPLICATION attribute and nothing more.
set -e
echo "host replication all all scram-sha-256" >> "$PGDATA/pg_hba.conf"
