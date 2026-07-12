# Recovery

## Disk-full condition

When the database or WAL grows past configured thresholds, Binnacle enters a degraded persistence state:

- **Warning** — expired data is cleaned aggressively.
- **Critical** — additional expired cleanup runs.
- **Emergency** — raw 10-second persistence pauses; rollups, settings, and events are preserved.

The live Metrics Engine and SSE continue to work during storage pressure. Free disk space or reduce retention, then restart Binnacle. Persistence resumes automatically once the budget is below emergency level.

## Corruption

If startup migration fails with an integrity error:

1. Stop the container.
2. Copy the database files to a safe location:

   ```bash
   docker cp binnacle:/var/lib/binnacle /tmp/binnacle-recovery
   ```

3. Attempt an integrity check on a copy:

   ```bash
   sqlite3 /tmp/binnacle-recovery/binnacle.db "PRAGMA integrity_check;"
   ```

4. If the database is corrupt, restore from your most recent backup or start with a fresh database. Binnacle does not automatically repair or delete a corrupt database.

## Consistent SQLite copy

To back up or inspect the database while Binnacle is running, use SQLite's online backup rather than copying open files:

```bash
docker exec binnacle sqlite3 /var/lib/binnacle/binnacle.db ".backup /var/lib/binnacle/binnacle-backup.db"
docker cp binnacle:/var/lib/binnacle/binnacle-backup.db ./binnacle-backup.db
```

## Reset monitoring history

From the Settings page you can delete history for one resource, data before a date, or all monitoring history. These operations require typed confirmation and run in bounded batches. They do not delete users or configuration.

## Restart and logs

```bash
docker compose -f packaging/docker/docker-compose.yml restart binnacle
docker compose -f packaging/docker/docker-compose.yml logs -f binnacle
```

Check `level=ERROR` entries for migration, disk, or collector failures.
