# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## Unreleased

- fix: rebuild image from post-bump tree (v0.2.0 image was built from pre-bump code and lacks go-release type support)

## v0.2.0

- bump github.com/bborbe/notification to v0.2.0 (adds go-release notification type support)

## v0.1.0

- feat: extract notification controller service from trading monorepo (consumes notification topic, routes to channel handlers)
