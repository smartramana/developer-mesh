# Swagger Documentation - GitHub Pages Deployment

This directory contains scripts for building and testing the Developer Mesh API documentation locally. The actual deployment to GitHub Pages happens automatically via GitHub Actions.

## Overview

- **Hosted at**: https://docs.dev-mesh.io
- **Deployment**: Automatic via GitHub Actions on push to main branch
- **Source**: `/docs/swagger/openapi.yaml`

## How It Works

1. **Automatic Deployment**: When you push changes to any file in `docs/swagger/**` on the main branch, GitHub Actions automatically:
   - Validates the OpenAPI specification
   - Builds the Swagger UI static site
   - Deploys to GitHub Pages

2. **GitHub Pages**: The documentation is served from GitHub Pages with:
   - Free HTTPS via GitHub's SSL certificates
   - Custom domain support (docs.dev-mesh.io)
   - Automatic CDN distribution

## Local Testing

Use the build script to test documentation changes locally before pushing:

```bash
# Build the docs locally
./scripts/swagger-docs/build-local.sh

# Serve locally
cd build/swagger-docs
python3 -m http.server 8000

# Visit http://localhost:8000
```

## Making Changes

1. Edit the OpenAPI specification files in `docs/swagger/`
2. Test locally using the build script
3. Commit and push to main branch
4. GitHub Actions will automatically deploy within minutes

## GitHub Actions Workflow

The deployment is handled by `.github/workflows/deploy-swagger-docs.yml`:
- Triggers on changes to `docs/swagger/**`
- Uses GitHub's official pages deployment action
- Validates OpenAPI spec before deployment
- Builds fresh on every deployment

## DNS Configuration

DNS records have been configured in Route53:
- A records pointing to GitHub Pages IPs
- AAAA records for IPv6 support
- CNAME file included in build for custom domain

## Repository Settings

Ensure these GitHub repository settings:
1. Go to Settings > Pages
2. Source: GitHub Actions (not branch)
3. Custom domain: docs.dev-mesh.io
4. Enforce HTTPS: ✓ (checked)

## Troubleshooting

### Changes Not Appearing
- Check GitHub Actions tab for deployment status
- Deployments can take 2-10 minutes
- Clear browser cache if needed

### Build Failures
- Check GitHub Actions logs
- Validate OpenAPI spec: `swagger-cli validate docs/swagger/openapi.yaml`
- Ensure all referenced files exist

### Custom Domain Issues
- DNS propagation can take up to 24 hours
- Verify DNS with: `dig docs.dev-mesh.io`
- Check CNAME file is present in build

## Notes

- No manual deployment needed - everything is automated
- The `gh-pages` branch is managed by GitHub Actions
- Don't manually edit the gh-pages branch
- Swagger UI version is defined in the workflow file