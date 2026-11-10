---
status: completed
spec: [001-cluster-build-arm]
summary: Added a BUILD_TARGET dispatch to Makefile.docker that preserves the byte-for-byte local build/upload/clean path and routes non-local targets to a guarded cluster build Job, plus a CHANGELOG Unreleased entry.
execution_id: 287451db-b92e-5c7c-bc3a-e230eb05ddbc-exec-001-spec-001-makefile-dispatch
dark-factory-version: dev
created: "2026-11-10T10:20:00Z"
queued: "2026-11-10T09:36:10Z"
started: "2026-11-10T09:36:14Z"
completed: "2026-11-10T09:44:35Z"
branch: dark-factory/cluster-build-arm
---

# Add the BUILD_TARGET dispatch to Makefile.docker

<summary>
- The local build command and its tag stay exactly as they are today.
- Setting a build target other than `local` routes the build into that cluster instead of the local Docker daemon.
- The cluster path demands an explicit version tag and refuses to run without one, before any cluster object is created.
- Both build paths publish to the same image repository, whose name is defined once so the two cannot drift.
- Uploading and cleaning become harmless no-ops on the cluster path, because the cluster already pushed the image and leaves nothing on this host.
- The operator's local publish command still uses the local path.
- No version/tag guard is added to the local path, so existing non-release checkouts keep building.
</summary>

<objective>
Split `make build` into two arms selected by `BUILD_TARGET`, keeping the existing local arm byte-for-byte equivalent in command and tag while adding a cluster arm that renders and runs a Kubernetes build Job. This makes the cluster build reachable; without the dispatch nothing invokes the runner the next prompt adds.
</objective>

<context>
Read before editing:
- `Makefile` — includes `Makefile.variables`, `Makefile.precommit`, `Makefile.docker`, then assigns `SERVICE = notification-controller` *after* the include. This ordering is why the image name must be recursive.
- `Makefile.docker` — the file being changed; currently has only the local build, upload, clean, apply, buca targets.
- `docs/dod.md` — the project definition of done.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-makefile-commands.md` — make target conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-dockerfile-guide.md` — image build conventions.

Reference dispatch shape (do not copy its extra knobs): the sibling repo `bborbe/agent-task-executor`'s `Makefile.docker` uses `BUILD_TARGET ?= local`, an `ifeq ($(BUILD_TARGET),local)` recipe split, and `$(origin VERSION)` to detect a caller-supplied tag. This repo deliberately omits that sibling's `check-version-tag`, `ALLOW_UNTAGGED_BUILD`, `ALLOW_DEFAULT_VERSION`, `BUILD_REPO_URL`, `BUILD_PUSH_SECRET` and `BUILD_DOCKERFILE` knobs.

Prompt 2 creates `hack/build-cluster.sh`. Until it lands, the cluster branch of this dispatch is inert; do not create `hack/` in this prompt.
</context>

<requirements>
1. In `Makefile.docker`, keep `DOCKER_REGISTRY ?= docker.io` and the existing `VERSION` default block unchanged:
```make
ifeq ($(VERSION),)
	VERSION := $(shell git describe --tags `git rev-list --tags --max-count=1`)
endif
```

2. Add the image name exactly once, immediately after `DOCKER_REGISTRY`, using a recursive assignment (not `:=`) because `SERVICE` is assigned in `Makefile` after this file is included:
```make
IMAGE ?= bborbe/$(SERVICE)
```

3. Add the dispatch knobs. Only `BUILD_TARGET` and the timeout are knobs; the repository URL and push-secret name are literals in the recipe, not configurable inputs:
```make
BUILD_TARGET ?= local
BUILD_TIMEOUT ?= 3600
```

4. Replace the `build` target with a two-arm dispatch. The local arm must produce the identical command and identical tag as today (`$(DOCKER_REGISTRY)/$(IMAGE):$(VERSION)` renders to `docker.io/bborbe/notification-controller:<version>`). The cluster arm must (a) refuse when `VERSION` was defaulted by this file, before invoking anything, (b) print the composed image reference, and (c) invoke `hack/build-cluster.sh` with the runner's environment contract. Use this body:
```make
.PHONY: build
build: check-go-mod
ifeq ($(BUILD_TARGET),local)
	@if [ -z "$(VERSION)" ]; then echo "ERROR: no VERSION set and no git tag found - set VERSION=vX.Y.Z or create a tag" >&2; exit 1; fi
	DOCKER_BUILDKIT=1 \
	docker build \
	--pull \
	--no-cache \
	--rm=true \
	--platform=linux/amd64 \
	--build-arg DOCKER_REGISTRY=$(DOCKER_REGISTRY) \
	--build-arg BUILD_GIT_COMMIT=$$(git rev-parse --short HEAD) \
	--build-arg BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
	-t $(DOCKER_REGISTRY)/$(IMAGE):$(VERSION) \
	-f Dockerfile .
else
# The cluster arm builds a caller-named ref and pushes it. It refuses to run
# without an explicit VERSION: the default above resolves to the newest release
# tag, and building over that tag would replace a released image.
	@if [ "$(origin VERSION)" = "file" ]; then \
		echo "ERROR: the cluster arm needs an explicit VERSION." >&2; \
		echo "  Pass VERSION=<scratch-tag> on the command line." >&2; \
		exit 1; \
	fi
	@echo "build-cluster: building $(DOCKER_REGISTRY)/$(IMAGE):$(VERSION) on $(BUILD_TARGET)"
	BUILD_TARGET='$(BUILD_TARGET)' \
	REPO_URL='https://github.com/$(IMAGE).git' \
	REF='$(REF)' \
	TAG='$(VERSION)' \
	DOCKERFILE='Dockerfile' \
	IMAGE='$(DOCKER_REGISTRY)/$(IMAGE)' \
	DOCKER_REGISTRY='$(DOCKER_REGISTRY)' \
	PUSH='true' \
	PUSH_SECRET='buildkit-push' \
	TIMEOUT='$(BUILD_TIMEOUT)' \
	hack/build-cluster.sh
endif
```
   Notes on the contract:
   - `REPO_URL`, `DOCKERFILE` and `PUSH_SECRET` are literals. Do not promote them to `?=` variables: a configurable clone source on a Job that mounts a push credential would let a caller point the credential at another repository.
   - `IMAGE` passed to the runner is the repository *without* a tag; the runner appends `TAG`.
   - `$(origin VERSION)` is `file` when the block above defaulted it and `command line` when the caller passed it.

5. Keep `check-go-mod` on the local branch only. Wrap its existing recipe body in the same conditional so it does nothing for a cluster build:
```make
.PHONY: check-go-mod
check-go-mod:
ifeq ($(BUILD_TARGET),local)
	@if [ -f "go.mod" ]; then \
		echo "go.mod found, running go mod vendor..."; \
		go mod vendor; \
	else \
		echo "go.mod not found, skipping go mod vendor."; \
	fi
endif
```

6. Guard `upload` and `clean` with `ifeq ($(BUILD_TARGET),local)` around the recipe body, not a recipe-level `exit 1`. The cluster branch prints a no-op message:
```make
.PHONY: upload
upload:
ifeq ($(BUILD_TARGET),local)
	docker push $(DOCKER_REGISTRY)/$(IMAGE):$(VERSION)
else
	@echo "upload: nothing to do - the cluster arm already pushed the image"
endif

.PHONY: clean
clean:
ifeq ($(BUILD_TARGET),local)
	docker rmi $(DOCKER_REGISTRY)/$(IMAGE):$(VERSION) || true
	docker image prune -f || true
	docker builder prune --all -f || true
	rm -rf vendor
else
	@echo "clean: nothing to do - the cluster arm leaves nothing on this host"
endif
```

7. Leave `apply` and `buca` unchanged (`apply` stays the publish-only no-op placeholder; `buca: build upload clean apply` remains the operator's local publish path with the default `BUILD_TARGET=local`). Add only a short comment above `buca` noting it is the local path.

8. Do not add a `Makefile.env`, a `check-version-tag` target, or any `ALLOW_*` variable. Do not add any other build knob.
</requirements>

<constraints>
- The local arm's command and tag must not change. `check-go-mod` stays on the local branch only: the cluster arm builds a named ref, which need not be the local `HEAD`, so preparing the local working tree there would be meaningless.
- `check-version-tag` is **not** copied from the pilot. It refuses to build `$(VERSION)` from a tree not checked out at that tag, and this repo has no such target today; copying it would break the local arm for any non-release checkout. `ALLOW_UNTAGGED_BUILD` is therefore not copied either.
- `IMAGE ?= bborbe/$(SERVICE)` is defined once, in `Makefile.docker`. `SERVICE` is assigned in `Makefile` *after* the include, so the definition must be recursive (`?=`), not `:=`.
- The `Makefile.env` guard is absent from this repo and must stay absent: nothing narrows, nothing is added.
- `upload` and `clean` are guarded by `ifeq ($(BUILD_TARGET),local)` around the recipe body — not a recipe-level `exit 1` — so `make -n` shows the local path and the cluster path does nothing.
- `ALLOW_DEFAULT_VERSION` is not copied: it would reintroduce an opt-out on the refusal this change ships.
- The repository URL and the push-secret name are literals in the dispatch, not configurable inputs.
- `IMAGE`, `SERVICE`, the registry and the tag must each satisfy the runner's character whitelist; no value may carry a newline.
- No new scenario. The behaviour is reachable by checks on the rendered invocation plus one real cluster build.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from the repository root (pass `ROOTDIR=/workspace` on every make call because the worktree workflow masks `.git`):

```bash
# dispatch exists
grep -q '^BUILD_TARGET ?= local$' Makefile.docker
grep -qxF 'IMAGE ?= bborbe/$(SERVICE)' Makefile.docker
grep -qE '^[[:space:]]*@?(\./)?hack/build-cluster\.sh' Makefile.docker

# local arm: identical command and tag
ROOTDIR=/workspace make -n build VERSION=probe | grep -q -- '-t docker.io/bborbe/notification-controller:probe'
ROOTDIR=/workspace make -n upload VERSION=probe | grep -q 'docker push docker.io/bborbe/notification-controller:probe'
ROOTDIR=/workspace make -n clean VERSION=probe | grep -q 'docker rmi docker.io/bborbe/notification-controller:probe'

# cluster arm: composed reference and the runner invocation are visible
ROOTDIR=/workspace make -n build BUILD_TARGET=sun REF=abc1234 VERSION=probe | grep -q 'docker.io/bborbe/notification-controller:probe'
ROOTDIR=/workspace make -n build BUILD_TARGET=sun REF=abc1234 VERSION=probe | grep -q 'hack/build-cluster.sh'

# cluster upload/clean are no-ops
ROOTDIR=/workspace make -n upload BUILD_TARGET=sun VERSION=probe | grep -q 'nothing to do'
ROOTDIR=/workspace make -n clean BUILD_TARGET=sun VERSION=probe | grep -q 'nothing to do'

# omitted VERSION renders the refusal, naming VERSION, before any runner call
ROOTDIR=/workspace make -n build BUILD_TARGET=sun REF=abc1234 | grep -q 'explicit VERSION'

# repository gate
ROOTDIR=/workspace make test
```

Before finishing, re-run every command above and confirm it passes, then walk each requirement against the edited file.
</verification>
