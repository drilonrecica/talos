# Uninstall

Removing TALOS removes the running container and, unless you preserve the volume, all historical monitoring data and configuration stored in SQLite.

## Docker Compose

```bash
cd talos
docker compose -f packaging/docker/docker-compose.yml down -v
```

The `-v` flag deletes the named `talos-data` volume. Omit it to keep the data:

```bash
docker compose -f packaging/docker/docker-compose.yml down
```

## Coolify

Delete the TALOS resource in Coolify. Choose whether to remove the persistent storage when prompted.

## Manual cleanup

```bash
docker stop talos
docker rm talos
docker volume rm talos-data
```

## Data consequences

TALOS does not include built-in backups. Deleting the volume removes:

- Historical host, resource, and event data.
- Rollup tables and retention settings.
- Administrator account and session records.
- Encrypted UI secrets (the master key must also be destroyed separately).

Container images and runtime logs on the host are not affected.
