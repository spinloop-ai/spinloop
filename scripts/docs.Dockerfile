# The image for scripts/docs-serve.sh. Build context is the repository root;
# the script mounts the working tree over /src, so this copy only serves the
# standalone case (docker run spinloop-docs serve).
FROM python:3.12-slim

# Keep in step with the pin in .github/workflows/docs.yml.
RUN pip install --no-cache-dir "mkdocs-material==9.7.7"

WORKDIR /src
COPY . .

ENTRYPOINT ["mkdocs"]
CMD ["serve", "-a", "0.0.0.0:8000"]
