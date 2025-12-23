# Hosting PixivFE

This guide covers hosting PixivFE using [Docker](#docker) (recommended) or a pre-compiled [binary](#binary).

!!! note
    PixivFE requires a pixiv session cookie to authenticate with the API. See [Authentication for the pixiv API](api-authentication.md) for instructions.

## Docker

Docker images are available from our [Container Registry on GitLab](https://gitlab.com/pixivfe/pixivfe/container_registry) for `linux/amd64` and `linux/arm64`. The `latest` tag points to the most recent stable release, while `next` points to the latest development build.

To start, clone the repository and enter the `deploy` directory:

```bash
git clone https://codeberg.org/PixivFE/PixivFE.git && cd PixivFE/deploy
```

Next, copy `.env.example` to `.env` and configure it. For more details, see [Configuration options](configuration-options.md).

!!! note "Set PIXIVFE_HOST for Docker"
    You must set `PIXIVFE_HOST=0.0.0.0` in your `.env` file. This allows the application inside the container to be accessible to Docker's networking layer.

With the configuration ready, start the container. The host will listen on `127.0.0.1:8282` by default.

!!! warning
    The Docker Compose command requires the [Compose plugin](https://docs.docker.com/compose/install).

=== "Docker Compose"
    ```bash
    docker compose up -d
    ```

=== "Docker CLI"
    ```bash
    docker run -d --name pixivfe -p 127.0.0.1:8282:8282 --env-file .env registry.gitlab.com/pixivfe/pixivfe:latest
    ```

You can view container logs with `docker logs -f pixivfe`.

## Binary

You can run PixivFE directly using a pre-compiled binary. We recommend using [Caddy](https://caddyserver.com/) as a reverse proxy, and downloading a pre-compiled version for your platform from our [Package Registry on GitLab](https://gitlab.com/pixivfe/PixivFE/-/packages). Make sure the downloaded file is executable.

Alternatively, you can build it from source:
```bash
git clone https://codeberg.org/PixivFE/PixivFE.git && cd PixivFE
./build.sh build
```

Next, you need to configure the application. Copy `deploy/.env.example` from the repository to a new `.env` file in the same directory as your binary. Edit the `.env` file as needed, referring to [Configuration options](configuration-options.md).

Once configured, run the application.
```bash
# If you downloaded a binary (example for linux/amd64)
./pixivfe-linux-amd64

# If you built from source, you can use the helper script
./build.sh run
```
PixivFE will be accessible at `localhost:8282` by default.

For a production setup with HTTPS, [install Caddy](https://caddyserver.com/docs/install) and create a `Caddyfile` to reverse proxy requests to PixivFE.
```caddy
example.com {
  reverse_proxy localhost:8282
}
```
Replace `example.com` with your domain and run `caddy run`.

## Updating

Follow the instructions for your deployment method to update to the latest version.

### Docker

If you are using Docker, first pull the latest image and any repository changes.

For **Docker Compose**, pull the image and restart the service:
```bash
docker compose pull && git pull
docker compose up -d
```

For the **Docker CLI**, pull the image, then stop, remove, and recreate the container:
```bash
docker pull registry.gitlab.com/pixivfe/pixivfe:latest && git pull
docker stop pixivfe && docker rm pixivfe
docker run -d --name pixivfe -p 127.0.0.1:8282:8282 --env-file .env registry.gitlab.com/pixivfe/pixivfe:latest
```

Per [recommendation from Go](https://tip.golang.org/doc/gc-guide#Suggested_uses), you may [set a memory limit](https://kupczynski.info/posts/go-container-aware/) for the container as follows:

```bash
docker run -d --name pixivfe -p 127.0.0.1:8282:8282 --env-file .env --memory=2000m -e GOMEMLIMIT=1800m registry.gitlab.com/pixivfe/pixivfe:latest
```

### Binary

To update a binary installation, get the latest binary by downloading it from the [Package Registry](https://gitlab.com/pixivfe/PixivFE/-/packages) or by rebuilding from source.

```bash
# To rebuild from source
git pull
./build.sh build
```

Then, restart the PixivFE process.

## Acknowledgements

- [Keep Caddy Running](https://caddyserver.com/docs/running#keep-caddy-running)
