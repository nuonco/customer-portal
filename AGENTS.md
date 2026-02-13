@README.md

# Agent-Specific Context

## Local Dev

You can assume I am already running this service in another terminal using `nuonctl dev`, which will use the commands defined in the `service.yml`. The commands there will rebuild and restart this service as you make changes, so there's no need to build the service yourself, restart it, run tests, etc. Anything that I want to happen during local dev is already configured in the `service.yml`.

`nuonctl dev` will create logs for each service it runs, at `/tmp/nuonctl-<service_name>`. So for customer-dashboard, the logs can be tailed at `/tmp/nuonctl-customer-dashboard`. This is useful if there are server-side errors that need to be debugged and fixed.

Nuonctl also runs a web UI at `localhost:7777`, which you can open in Chrome if you need to see the status of all the services `nuonctl` is running.

## Tests and Documentation

When making changes to the app, ensure that you are updating and adding to the tests. Also make sure to update the README.md to keep the content there up-to-date. Include these changes in your plans.
