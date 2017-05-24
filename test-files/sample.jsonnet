local k = import "k.libsonnet";
local container = k.core.v1.container;
local deployment = k.apps.v1beta1.deployment;

deployment.default("nginx",
                   container.default("web", "nginx:1.13.0") +
                   container.helpers.namedPort("www", 80))
