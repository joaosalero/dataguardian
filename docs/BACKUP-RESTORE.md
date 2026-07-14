# Backup and Restore

DataGuardian stores PostgreSQL data and inspected artifacts in Docker volumes. Backups may contain sensitive or malicious material; encrypt them and restrict access.

Stop writes before backup:

```sh
docker compose stop backend-go frontend
docker compose exec -T db pg_dump -U dataguardian -d dataguardian > dataguardian.sql
docker run --rm -v dataguardian_backend_uploads:/data -v "$PWD":/backup alpine tar czf /backup/dataguardian-uploads.tgz -C /data .
docker compose start backend-go frontend
```

Do not commit either backup. Restore only into an isolated trusted deployment, verify checksums first, and never open backed-up artifacts directly.
