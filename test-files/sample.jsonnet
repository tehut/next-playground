local k = import "ksonnet.beta.2/k.libsonnet";
local deployment = k.apps.v1beta1.deployment;
local container = deployment.mixin.spec.template.spec.containersType;
local containerPort = container.portsType;

local nginxContainer =
  container.new("web", "nginx:1.13.0") +
  container.ports(containerPort.newNamed("www", 80));

deployment.new("nginx", 2, nginxContainer)
