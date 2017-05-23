# ksonnet Playground

This is the site that drives https://ksonnet-playground.heptio.com.

## CI/CD

- PR's are tested by running `make ci-test`, which builds the docker image and runs `ci/test.sh`
- All git tags are built and pushed to `gcr.io/heptio-images/ksonnet-playground:$TAG` [![Build Status](https://jenkins.i.heptio.com/buildStatus/icon?job=ksonnet-playground-tag-deployer)](https://jenkins.i.heptio.com/job/ksonnet-playground-tag-deployer)
- Deployment is handled via the slack command `/deploy-ksonnet-playground <tag>` (tags must already be pushed by the above job first)
