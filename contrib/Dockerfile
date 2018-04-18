FROM envoyproxy/envoy:latest
ADD envoy.yaml /envoy.yaml

CMD /usr/local/bin/envoy -c /envoy.yaml --v2-config-only -l debug --service-node node --service-cluster cluster
