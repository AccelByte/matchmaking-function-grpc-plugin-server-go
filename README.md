# AccelByte Plugin Architecture Demo using Go [Server Part]

## Setup
This demo requires Go 1.18 to be installed.

1. For complete server components to work, you need `docker` and `docker-compose` to be installed.
2. Install docker logging driver for loki with this command:
    ```bash
    $ docker plugin install grafana/loki-docker-driver:latest --alias loki --grant-all-permissions
    ```
3. You can verify whether loki driver has been installed using:
    ```bash
    $ docker plugin ls
    ```
4. Go 1.18 is required to build and run outside of docker environment.

## Usage

1. Make an env file in the root directory `plugin-arch-grpc-server-go/.env` with value
    ```bash
    AB_USERNAME=user@mail.com
    AB_PASSWORD=pass123
    AB_NAMESPACE=test
    AB_CLIENT_ID=clientId123
    AB_CLIENT_SECRET=clientSecret123
    AB_BASE_URL=https://demo.accelbyte.io
    ```
3. Run dependencies first.
    ```bash
    $ docker-compose -f docker-compose-dependencies.yaml up
    ```
4. Then run app. Use `--build` if the app image need to be rebuild. For example when there are changes in configuration.
    ```bash
    $ docker-compose -f docker-compose-app.yml up --build
    or
    $ docker-compose -f docker-compose-app.yml up
    ```
5. Use Postman or any other Grpc client, and point it to `localhost:10000` (default). Grpc service discovery is already enabled and if client supported it, then it can be use to simplify the testing.