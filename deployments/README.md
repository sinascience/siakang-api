# Deployment Files

Folder ini berisi semua file yang diperlukan untuk deployment aplikasi Lakukan Backend.

## Struktur Folder

```
deployments/
├── README.md                      # Dokumentasi ini
├── docker/                        # Docker-related files
│   └── docker-entrypoint.sh       # Entrypoint script untuk container (migrations, etc)
└── kubernetes/                    # Kubernetes manifests
    └── k8s-deployment.yaml        # Complete K8s deployment (ConfigMap, Secret, Deployment, Service, HPA, Ingress)

# Di root project:
└── Dockerfile                     # Docker image definition
```

## Quick Links

### Kubernetes (Production)

File: [`kubernetes/k8s-deployment.yaml`](kubernetes/k8s-deployment.yaml)

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml

# Check status
kubectl get pods -l app=lakukan-api

# View logs
kubectl logs -f deployment/lakukan-api
```

### CI/CD

File: [`../.ci/Jenkinsfile`](../.ci/Jenkinsfile)

Jenkins pipeline untuk automated build dan deployment.

## File Descriptions

### docker-entrypoint.sh

Script yang dijalankan saat container start. Fungsi utama:
- ✅ Wait for database connection (max 5 minutes)
- ✅ Run database migrations (core & CRM)
- ✅ Start aplikasi
- ✅ Support `SKIP_MIGRATION=true` environment variable

**Digunakan oleh:**
- Docker Compose (local development)
- Kubernetes pods (production)

### docker-compose.yml

Orchestration untuk local development dengan:
- PostgreSQL database
- API service dengan auto-migrations

**Environment variables:** Loaded from root `.env` file

### k8s-deployment.yaml

Complete Kubernetes deployment dengan:
- **ConfigMap**: Non-sensitive configuration
- **Secret**: Database credentials, JWT secret, SMTP config
- **Deployment**: 2 replicas dengan rolling update strategy
- **Service**: ClusterIP untuk internal access
- **HorizontalPodAutoscaler**: Auto-scaling 2-10 pods
- **Ingress**: External access dengan HTTPS

**IMPORTANT:** Update secrets sebelum deploy ke production!

## Usage by Environment

### Staging/Production (Kubernetes)

```bash
# 1. Update secrets
vim deployments/kubernetes/k8s-deployment.yaml

# 2. Apply manifests
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml

# 3. Verify deployment
kubectl get all -l app=lakukan-api
```

### CI/CD (Jenkins)

```bash
# Jenkins will automatically:
# 1. Checkout code
# 2. Build Docker image (no DB connection needed!)
# 3. Push to registry
# 4. Deploy to Kubernetes
# 5. Verify deployment

# Jenkinsfile location: .ci/Jenkinsfile
```

## Environment Variables

### Required for Production

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `postgres.example.com` |
| `DB_PASSWORD` | Database password | `secure-password-here` |
| `JWT_SECRET` | JWT signing key (min 32 chars) | `your-super-secret-key...` |
| `SMTP_HOST` | SMTP server | `smtp.gmail.com` |
| `SMTP_USER` | SMTP username | `noreply@example.com` |
| `SMTP_PASSWORD` | SMTP password | `your-smtp-password` |
| `FRONTEND_URL` | Frontend URL | `https://app.example.com` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `SKIP_MIGRATION` | Skip migrations on startup | `false` |
| `ENV` | Environment mode | `production` |
| `DB_SSLMODE` | PostgreSQL SSL mode | `require` |

## Troubleshooting

### Migrations Failed

```bash
# Check logs
docker-compose -f deployments/docker/docker-compose.yml logs api

# Or in Kubernetes
kubectl logs -f deployment/lakukan-api

# Run migrations manually
docker-compose -f deployments/docker/docker-compose.yml exec api \
  /usr/local/bin/docker-entrypoint.sh echo "done"
```

### Skip Migrations

```bash
# Docker Compose
docker-compose run -e SKIP_MIGRATION=true api

# Kubernetes - add to deployment env
kubectl set env deployment/lakukan-api SKIP_MIGRATION=true
```

## Related Documentation

- [Docker Setup Guide](../README.Docker.md) - Comprehensive Docker & Kubernetes documentation
- [Main README](../README.md) - Project overview and development guide
- [Makefile](../Makefile) - Build commands and database management

## Best Practices

### Security
- ✅ Never commit secrets to git
- ✅ Use Kubernetes Secrets for sensitive data
- ✅ Rotate JWT_SECRET regularly
- ✅ Enable SSL for database connections in production

### Performance
- ✅ Set appropriate resource limits in K8s
- ✅ Enable HPA for auto-scaling
- ✅ Use connection pooling (already configured)

### Reliability
- ✅ Use rolling updates for zero downtime
- ✅ Set proper health checks
- ✅ Run multiple replicas (minimum 2)
- ✅ Implement retry logic

## Support

For issues or questions:
- Check [Troubleshooting Guide](../README.Docker.md#troubleshooting)
- Review logs using commands above
- Contact DevOps team
