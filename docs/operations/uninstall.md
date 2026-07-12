# Uninstall

Removing Binnacle removes the running container and, unless you preserve the volume, all historical monitoring data and configuration stored in SQLite.

## Docker Compose

```bash
cd binnacle
docker compose -f packaging/docker/docker-compose.yml down -v
```

The `-v` flag deletes the named `binnacle-data` volume. Omit it to keep the data:

```bash
docker compose -f packaging/docker/docker-compose.yml down
```

## Coolify

Delete the Binnacle resource in Coolify. Choose whether to remove the persistent storage when prompted.

## Manual cleanup

```bash
docker stop binnacle
docker rm binnacle
docker volume rm binnacle-data
```

## Data consequences

Binnacle does not include built-in backups. Deleting the volume removes:

- Historical host, resource, and event data.
- Rollup tables and retention settings.
- Administrator account and session records.
- Encrypted UI secrets (the master key must also be destroyed separately).

Container images and runtime logs on the host are not affected.
