---
status: approved
approved: "2026-11-10T08:38:28Z"
branch: dark-factory/cluster-build-arm
---

## Summary

- This repo's image can currently be produced on one machine only: the Mac.
- Its sibling repos can also build their image inside a Kubernetes cluster, unattended, from any committed ref.
- Add that second build path here as an arm beside the existing local one, leaving the local command and tag as they are.
- The cluster arm builds a named commit as a Kubernetes Job and pushes the result to the same Docker Hub repository the local arm pushes to.
- Four supporting edits make the arm reachable: `IMAGE` must be defined, the image must build from a fresh clone, the pipeline's own log directories must be ignored, and the validation command must stop reaching the network.

## Problem

`notification-controller` runs on `nukedev` and `nukeprod`, and its StatefulSet pulls `bborbe/notification-controller` (`nuke/notification-controller/notification-controller-sts.yaml` renders `{{"REGISTRY_HOST" | env}}:443/bborbe/notification-controller:v0.7.0`). Its build files — `Dockerfile`, `Makefile`, `Makefile.docker` — carry no `BUILD_TARGET` arm, so that image can only be cut by hand at this Mac.

The arm is not a straight copy. Five properties of this repo make a naive paste fail at runtime rather than at review:

- **`IMAGE` is undefined.** `Makefile:5` declares only `SERVICE = notification-controller`. The copied dispatch composes `IMAGE='$(DOCKER_REGISTRY)/$(IMAGE)'` and `BUILD_REPO_URL ?= https://github.com/$(IMAGE).git`, so a verbatim copy renders `docker.io/` and `https://github.com/.git`; the Job's init container then fails to clone. The image this repo actually publishes is `$(DOCKER_REGISTRY)/bborbe/$(SERVICE)`, so `IMAGE ?= bborbe/$(SERVICE)` is the definition that matches the local arm.
- **`VERSION` defaults inside `Makefile.docker`.** Lines 3–4 set `VERSION := $(shell git describe --tags …)` when it is empty, so `$(origin VERSION)` reads `file` — the copied `= file` guard fires and forces an explicit tag. That is the desired behaviour, but it means `make build BUILD_TARGET=<cluster>` without `VERSION` is a refusal, not a build.
- **The Dockerfile builds with `-mod=vendor`.** `vendor/` is absent from `origin/master`, so a Job that clones the repo has no vendor directory and the build dies before it starts. The local arm hides this because `check-go-mod` runs `go mod vendor` on the host first.
- **`.gitignore` is `data/` plus `*.log`.** That does not ignore `specs/log/*.jsonl`, which dark-factory writes on every prompt, nor `vendor/`, which `check-go-mod` creates. A dark-factory run on this repo would commit its own transcripts.
- **No `Makefile.env` branch guard exists.** Unlike the pilot repo, there is nothing here that refuses `build` on a non-`dev`/`prod` branch, so nothing needs narrowing — and nothing may be added.

Two further properties shape the change rather than block it:

- **There is no README.** `docs/dod.md` is the only in-repo documentation of the build path, so the two-arm topology has no natural prose home and is not invented for this change.
- **`.dark-factory.yaml` already exists** and its `validationCommand` is unset, so it defaults to `make precommit` — which chains `lint vet vulncheck osv-scanner trivy`, and `vulncheck`/`osv-scanner` fetch modules over the network while `trivy` pulls `ghcr.io/aquasecurity/trivy-db`.

## Goal

`make build` on this repo produces the image locally exactly as it does today, and `make build BUILD_TARGET=<cluster> REF=<sha> VERSION=<tag>` produces it inside that cluster as a Kubernetes Job, pushing `docker.io/bborbe/notification-controller:<tag>` — the Docker Hub repository the `buildkit-push` credential authenticates against. Both arms work from the same checkout, and the cluster arm works from a fresh clone with no local preparation.

## Non-goals

- **Rolling out a new image.** The StatefulSet pins `v0.7.0` and the private-registry copy the deployment pulls is produced by nuke's `mirror:` step, not by this build. This spec adds the capability; nothing is deployed.
- **Changing the local arm's command or its tag.** Both stay as they are. The Dockerfile does move to module resolution, because the cluster arm cannot build from a clone otherwise. That does change how the *local* arm fetches dependencies: with `-mod=vendor` it built from the `vendor/` directory `check-go-mod` creates, and with `-mod=mod` the `golang:1.27.3` build stage downloads them from the module proxy. `check-go-mod` is kept on the local arm because it also verifies the module graph; it no longer feeds the image build.
- **Renaming the image** or adding a `bborbe/` prefix that does not already exist. The local arm already publishes `bborbe/$(SERVICE)` and the deployment consumes that name.
- **Touching the other repos** in the migration set — each carries its own task.
- **Building at release or tag time.** Auto-Build Docker Images on Release owns *when* a build is triggered.
- **Multi-arch images.** `linux/amd64` only, matching the cluster nodes.
- **Extra build knobs** beyond `BUILD_TARGET`, `IMAGE`, `VERSION` and the timeout. `ALLOW_DEFAULT_VERSION` is not copied: it would reintroduce an opt-out on the refusal this spec ships.
- **Adding a README.** The repo has none today and this change does not create one.

## Acceptance Criteria

- [ ] **The local arm's command and tag are unchanged and still work** — `DOCKER_CONTEXT=orbstack make build` exits 0, and `DOCKER_CONTEXT=orbstack docker image inspect docker.io/bborbe/notification-controller:<version>` exits 0. Evidence: the exit code and the inspected `.Id`. ⚠️ The `DOCKER_CONTEXT=orbstack` prefix is load-bearing: the machine's active context is `sun` (measured 2026-11-10), so a bare `make build` can build on the cluster and satisfy this box while the local arm is broken.
- [ ] **Post-Deploy (Rung-2):** the cluster arm builds and pushes from a fresh clone — `make build BUILD_TARGET=<cluster> REF=<full 40-character sha> VERSION=<scratch-tag>` exits 0, and `crane digest docker.io/bborbe/notification-controller:<scratch-tag>` resolves to a digest equal to the Job's own `containerimage.digest`. Evidence: the exit code, the Job name, the build pod's `spec.nodeName`, both digest values, and `DOCKER_CONTEXT=orbstack docker image inspect` on the same reference failing on this Mac with `No such image` — a stopped daemon or a malformed reference does not satisfy this.
  - `deploy_check:` `kubectl --context default --kubeconfig ~/.kube/sun -n build get secret buildkit-push -o jsonpath='{.metadata.name}'`
  - `deploy_target:` `buildkit-push`
- [ ] **The cluster arm pushes to the repository the deployment consumes** — the dispatch renders `docker.io/bborbe/notification-controller:<tag>` as the image reference, so `make -n build BUILD_TARGET=<cluster> REF=<sha> VERSION=probe` shows that exact string and no empty repository path. Evidence: the printed reference, and the digest above resolving at exactly that repository.
- [ ] **`hack/` holds the five files and the cluster branch invokes the runner** — at `HEAD`, `git ls-tree --name-only HEAD hack/` lists `build-cluster.sh`, `cluster-alias.sh`, `build-job.yaml`, `build-cluster-bootstrap.sh` and `dockerconfig.json.tpl`; each satisfies `test -s`; `grep -c '^BUILD_TARGET' Makefile.docker` returns ≥ 1 and `grep -cE '^[[:space:]]*@?(\./)?hack/build-cluster\.sh' Makefile.docker` returns ≥ 1 on an invocation line rather than a comment. Evidence: the five names with their byte sizes and both counts.
- [ ] **The cluster arm refuses a tag the caller did not choose** — `make build BUILD_TARGET=<cluster> REF=<sha>` with no `VERSION` on the command line exits non-zero and prints a message naming `VERSION`, before any Job is applied. Evidence: the exit code, the message, and `kubectl -n build get jobs` showing no new Job for that run.
- [ ] **The image builds from a fresh clone** — `grep -c '\-mod=vendor' Dockerfile` returns 0 and `grep -c '\-mod=mod' Dockerfile` returns ≥ 1, and `.gitignore` ignores `vendor/` and the pipeline's log directories while leaving the spec and prompt markdown tracked. Evidence: both Dockerfile counts, `git check-ignore -v --no-index vendor/probe` exiting 0, `git check-ignore -v --no-index specs/log/probe.jsonl` exiting 0, `git check-ignore -v --no-index specs/cluster-build-arm.md` exiting 1, and `ls -d prompts/in-progress prompts/completed prompts/log specs/in-progress specs/completed specs/log` succeeding.
- [ ] **The pipeline gates run without vulnerability scanners** — `.dark-factory.yaml` sets `preflightCommand: make test`, `testCommand: make test` and `validationCommand: make test lint`, so neither the host-side preflight nor a prompt is gated on `vulncheck` / `osv-scanner` module fetches or a `trivy` database pull. (`make test` and `make lint` still resolve modules through the proxy on a cold cache; that is accepted.) Evidence: `grep -c 'preflightCommand: make test' .dark-factory.yaml` returns 1, `grep -c 'validationCommand: make test lint' .dark-factory.yaml` returns 1, and `make -n test` exits 0. Evidence: `grep -c 'validationCommand: make test lint' .dark-factory.yaml` returns 1, and `make -n test lint` exits 0.
- [ ] **The CHANGELOG records the change** — `CHANGELOG.md` carries a `## Unreleased` section whose first bullet names the cluster build arm. Evidence: `grep -c '^## Unreleased$' CHANGELOG.md` returns 1 and the bullet text.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make test` — unit and integration suites pass
- `make lint` — golangci-lint clean
- `bash -n hack/build-cluster.sh && bash -n hack/cluster-alias.sh && bash -n hack/build-cluster-bootstrap.sh` — the copied runners parse
- `grep -c '^BUILD_TARGET' Makefile.docker` returns ≥ 1
- `grep -cE '^[[:space:]]*@?(\./)?hack/build-cluster\.sh' Makefile.docker` returns ≥ 1
- `git ls-tree --name-only HEAD hack/` lists the five expected files
- `grep -c '\-mod=vendor' Dockerfile` returns 0; `grep -c '\-mod=mod' Dockerfile` returns ≥ 1
- `git check-ignore -v --no-index vendor/probe` exits 0; `git check-ignore -v --no-index specs/log/probe.jsonl` exits 0; `git check-ignore -v --no-index specs/cluster-build-arm.md` exits 1
- `grep -c 'preflightCommand: make test' .dark-factory.yaml` returns 1; `grep -c 'validationCommand: make test lint' .dark-factory.yaml` returns 1
- `ls -d prompts/in-progress prompts/completed prompts/log specs/in-progress specs/completed specs/log` succeeds
- `make -n build BUILD_TARGET=sun REF=<sha> VERSION=probe` prints `hack/build-cluster.sh` and the composed `docker.io/bborbe/notification-controller:probe`
- `grep -c '^## Unreleased$' CHANGELOG.md` returns 1

### Operator-executable (runs on the host after PR merge, spec verification ladder)

- `DOCKER_CONTEXT=orbstack make build` then `DOCKER_CONTEXT=orbstack docker image inspect docker.io/bborbe/notification-controller:<version>` — the local arm still works
- `make build BUILD_TARGET=<cluster> REF=<sha> VERSION=<scratch-tag>` — one real cluster build, Job name, node and digest recorded
- `crane digest docker.io/bborbe/notification-controller:<scratch-tag>` — the push landed, and the value equals the Job's `containerimage.digest`
- `DOCKER_CONTEXT=orbstack docker image inspect docker.io/bborbe/notification-controller:<scratch-tag>` — must fail with `No such image`

## Desired Behavior

1. **A build target selects the arm.** `BUILD_TARGET` defaults to `local`. With `local`, `make build` behaves exactly as it does today: `check-go-mod` prepares the module cache, `docker build` runs on this machine, and the image is tagged `$(DOCKER_REGISTRY)/bborbe/$(SERVICE):$(VERSION)`.
2. **Any other value routes to a cluster Job.** With `BUILD_TARGET` set to a cluster alias — `sun` is the reference target this spec's ACs use — `make build` renders the Job template and applies it through `hack/build-cluster.sh`, passing the repository URL and the push-secret name as literals. It waits for the Job and reports success or failure with the Job's own log lines.
3. **The cluster arm needs a caller-supplied tag.** The cluster arm refuses to run unless `VERSION` came from the caller. Because `Makefile.docker` defaults `VERSION` from the newest release tag, a bare `make build BUILD_TARGET=<cluster>` would push over that released image; the refusal names `VERSION` and happens before any Job is created.
4. **Both arms publish to one repository.** The cluster arm composes `$(DOCKER_REGISTRY)/$(IMAGE):$(VERSION)` with `IMAGE ?= bborbe/$(SERVICE)`, matching the reference the local arm tags today, differing only in tag, and defined once so the two cannot drift.
5. **`upload` and `clean` stay local.** A cluster build pushes from inside the cluster and leaves nothing on this host, so both targets no-op rather than failing after a build that actually succeeded. `buca` remains the operator's local publish path.
6. **The cluster build works from a clone.** The image build resolves its Go dependencies in module mode, so a Job that clones the repo with no `vendor/` directory builds the same way a developer's checkout does.
7. **The pipeline's own files are ignored.** `specs/log` and `prompts/log` are ignored, as are `vendor/` and dark-factory's lock and log files; the spec and prompt markdown stay tracked. `data/` and `*.log` keep their current entries.
8. **The dark-factory validation command is network-free.** `.dark-factory.yaml` sets `validationCommand: make test lint` so a prompt is gated on the code, not on a module proxy or a vulnerability-database pull.

## Constraints

- The local arm's command and its tag must not change. `check-go-mod` stays on the local branch only: the cluster arm builds a named ref, which need not be the local `HEAD`, so preparing the local working tree there would be meaningless.
- `check-version-tag` is **not** copied from the pilot. That target refuses to build `$(VERSION)` from a tree not checked out at that tag, and this repo has no such target today; copying it would break the local arm for any non-release checkout (AC1). `ALLOW_UNTAGGED_BUILD` is therefore not copied either.
- `IMAGE ?= bborbe/$(SERVICE)` is defined once, in `Makefile.docker`. `SERVICE` is assigned in `Makefile` *after* the include, so the definition must be recursive (`?=`), not `:=`.
- The `Makefile.env` guard is absent from this repo and must stay absent: nothing narrows, nothing is added.
- `upload` and `clean` are guarded by `ifeq ($(BUILD_TARGET),local)` around the recipe body — not a recipe-level `exit 1` — so `make -n` shows the local path and the cluster path does nothing.
- The five `hack/` files are copied unmodified: `hack/build-cluster.sh`, `hack/cluster-alias.sh` and `hack/build-job.yaml` from `bborbe/agent-task-executor` at `origin/master` (its Job spec carries `automountServiceAccountToken: false`); `hack/build-cluster-bootstrap.sh` and `hack/dockerconfig.json.tpl` from `bborbe/go-skeleton` at `origin/master`. `cluster-alias.sh` is required — the runner sources it and fails at that line without it. `build-cluster-bootstrap.sh` and `dockerconfig.json.tpl` are copied because the arm-source decision requires the same five-file set in every repo of the migration; **neither is invoked by the build path** — the bootstrap provisions a new cluster's `build` namespace and `buildkit-push` secret and needs TeamVault tooling this repo does not carry, so here it is an unused aid and the namespace/secret is a stated Assumption. Do not wire it into `make build`. ⚠️ The YOLO container cannot reach a sibling repository, so the prompt must **inline all five file bodies verbatim**; a `git clone` of the arm source from inside the container is not available.
- The Dockerfile's only change is `-mod=vendor` → `-mod=mod`. `COPY . /workspace` and the `--build-arg DOCKER_REGISTRY` plumbing stay; the Job template already passes `DOCKER_REGISTRY=docker.io`, so the base images resolve from the public registry inside the cluster.
- `.gitignore` keeps `data/` and `*.log`, and adds `/vendor`, `/specs/log`, `/prompts/log`, `/.dark-factory.lock`, `/.dark-factory.log`, `/specs/in-progress/*.lock` and `/prompts/in-progress/*.lock`, with `.keep` files under `specs/`, `specs/in-progress/`, `specs/completed/`, `specs/log/`, `prompts/`, `prompts/in-progress/`, `prompts/completed/` and `prompts/log/` — the canonical six from `dark-factory/docs/init-project.md` § Step 4. ⚠️ That tree is load-bearing, not hygiene: `bborbe/openldap`'s first agent run failed `save prompt metadata: open prompts/in-progress/…: no such file or directory` after generating the prompts, because the directories did not exist.
- `.dark-factory.yaml` keeps `workflow: worktree`, `pr: true`, `autoRelease: false`, `autoGeneratePrompts: true` and `validationPrompt: docs/dod.md`, and adds **three** commands: `preflightCommand: make test`, `testCommand: make test`, `validationCommand: make test lint`. ⚠️ All three default to `make precommit`, which in this repo chains `trivy` (needs the `trivy` binary and a `ghcr.io/aquasecurity/trivy-db` pull) plus `vulncheck` / `osv-scanner` (network module fetches). `preflightCommand` runs **on the host before each prompt**, so a preflight that cannot run kills every prompt before the container starts — the failure `bborbe/openldap` measured. ⚠️ **Both bullets above are landed on this branch ahead of the lane** (`cd5e0e3`-style scaffolding commit): the directory tree and the three commands must exist before prompt *generation* runs, so they cannot be produced by a prompt. Measured on this repo 2026-11-10: the first agent run (`ab2f8885`) died `save prompt metadata: write file: open prompts/in-progress/001-spec-001-makefile-dispatch.md: no such file or directory` for exactly this reason.
- `IMAGE`, `SERVICE`, the registry and the tag must each satisfy the runner's character whitelist; no value may carry a newline.
- No new scenario. The behaviour is reachable by checks on the rendered invocation plus one real cluster build.

## Assumptions

These must hold before the cluster AC can pass; none is created by this spec.

- The `build` namespace and the `buildkit-push` dockerconfigjson secret exist in the target cluster, **and that credential authenticates against `docker.io`** — not against the private mirror. This is why the cluster arm pushes `docker.io/bborbe/notification-controller`; the private-registry copy the deployment pulls comes from nuke's `mirror:` step.
- The repo is public, so the Job's anonymous `git fetch` succeeds; the init container carries no clone credential.
- `crane` is installed on the host running the ACs.
- `REF` exists in the local repository **and on `origin`**: the runner derives `SOURCE_DATE_EPOCH` from the local repository, and the Job's init container fetches the ref from the remote. A commit that exists only locally fails at the clone.
- The build pod can reach the Go module proxy.
- `kubectl` is configured for the target cluster through the alias files `hack/cluster-alias.sh` resolves.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| `BUILD_TARGET` names an unknown cluster or context | The runner fails before applying anything and lists the accepted values | Re-run with a listed target; no Job exists to clean up |
| `VERSION` omitted on the cluster arm | Refusal before the Job is created, naming `VERSION` | Re-run passing `VERSION=<scratch-tag>` |
| `REF` is a short SHA, or is not in the local repository | The runner dies resolving `SOURCE_DATE_EPOCH` | `git fetch`, then re-run with the full 40-character SHA |
| `REF` exists locally but was never pushed | The clone init container fails: the ref does not exist on `origin` | Push the branch or commit, then re-run |
| The clone's commit epoch differs from the locally derived one | The build container refuses before `buildctl` runs and names both epochs | `git fetch`, then re-run |
| A Job with the derived name already exists in `build` | The runner refuses and names the existing Job rather than overwriting it | Delete the finished Job, then re-run |
| The runner is interrupted after `apply` | The Job keeps running in the cluster while the local invocation exits; the next run then hits the existing-Job refusal | Inspect `kubectl -n build get jobs`, read the pod's log, delete the Job once finished |
| The build pod cannot reach the module proxy | The Job fails; the runner prints the last 100 log lines and the Job name | Inspect the Job logs; the local arm is unaffected |
| The local build host cannot reach the module proxy | `make build` fails inside the `golang` stage while downloading modules — a regression from the `-mod=vendor` path, which built offline from `vendor/` | Restore proxy reachability, or warm the module cache before building; the cluster arm is unaffected |
| The build pod is OOM-killed or throttled | The Job reports `failed`; the pod status carries `OOMKilled` or a CPU-throttling event | Raise the template's limits in the same change, or build locally for that one image |
| The registry rejects the push | The build fails at the push step with the registry's own error | Re-check the `build` namespace secret, then re-run |
| A caller-supplied `VERSION` names an already-released tag | No guard — the push replaces that tag irreversibly | Re-push the image from the released tag, then cut a new release |
| The local arm regresses | `make build` fails or tags differently | Revert the arm; the local path is the regression gate |

## Security / Abuse Cases

- The runner substitutes caller-supplied values into a manifest and into embedded shell, so every value is checked against a character whitelist and rejected for embedded newlines before rendering. Do not add a value path that bypasses that check.
- The repository URL and the push-secret name are literals in the dispatch, not configurable inputs. A configurable clone source on a build that mounts a push credential would let a caller point the credential-bearing Job at another repository.
- The build Job mounts a push credential from a `build`-namespace secret and runs with `automountServiceAccountToken: false`. The credential is scoped to pushing an image; it grants nothing else.
- The build container runs as an unprivileged user with the sandbox profiles relaxed because rootless BuildKit requires it. That relaxation is deliberate and must not be copied to the clone init container.
- `VERSION` is caller-controlled and becomes an image tag and a Job name. The tag is whitelisted and the Job name is sanitized to a valid DNS name, truncated, and stripped of trailing separators.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `Makefile.docker` dispatch: the `BUILD_TARGET` split, `IMAGE ?= bborbe/$(SERVICE)`, the `VERSION` refusal, and local-only `upload`/`clean` | 1, 2, 3, 4, 5 | 1, 3, 5 | — |
| 2 | The build path itself: the five `hack/` files and the module-mode Dockerfile | 2, 6 | 2, 4, 6 | prompt 1 (the dispatch calls the runner) |
| 3 | Repo hygiene: the `.gitignore` layout with `.keep` files, the `.dark-factory.yaml` `validationCommand`, the CHANGELOG bullet | 7, 8 | 7, 8 | — |

Rationale: prompt 1 establishes the dispatch that makes any of it reachable; prompt 2 supplies the runner the dispatch calls and the clone-compatible image build; prompt 3 is independent and touches no build logic.

## Do-Nothing Option

Leaving this repo without a cluster arm keeps one maintained image tied to the Mac. Every rebuild, every release cut while the operator is away from this machine, and every future unattended pipeline needs a human at the keyboard for this one repo — while its siblings build themselves. The cost is not the one build; it is that the automation has a hole shaped exactly like the repo nobody is watching.
