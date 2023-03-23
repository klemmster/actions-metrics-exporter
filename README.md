# actions-metrics-exporter

docker run -d -p 9091:9091 prom/pushgateway

docker run \
    -p 9090:9090 \
    -v ./prometheus.yml:/etc/prometheus/prometheus.yml \
    prom/prometheus

# Dev Setup
## Forward events:
  Created a channel on smee.io
  https://smee.io/CXK7t0EH5kKuCmFk

  ```
  npm install -u smee-client`
  ```
  ```
    ~/node_modules/smee-client/bin/smee.js --url https://smee.io/CXK7t0EH5kKuCmFk --target http://127.0.0.1:8080/api/github/hook
  ```
##

