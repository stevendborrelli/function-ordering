# function-ordering

A Crossplane composition function that composes resources and declares
ordering constraints between them.

It exists to exercise [composed resource ordering][design], a prototype that
lets functions tell Crossplane one composed resource depends on another so it
can sequence what it creates, updates and deletes. No published function can
do that, because the protocol field is new — hence this one.

**Not intended for production use.** It composes placeholder resources and
reports readiness in whatever way makes ordering easiest to observe.

## Requirements

Crossplane built from the [composed resource ordering branch][branch], started
with `--enable-composed-resource-ordering`. Against any released Crossplane the
function still runs, but the dependencies it returns are ignored.

## Input

```yaml
apiVersion: ordering.fn.crossplane.io/v1alpha1
kind: Input

# Shorthand for a chain: each name depends on every name before it. Composed
# as well as ordered.
sequence: [vpc, subnet, instance]

# Composed without implying any order. Use with edges for arbitrary graphs.
resources: [vpc, subnet-a, subnet-b, instance]

# Explicit ordering constraints over resources.
edges:
- {resource: subnet-a, dependsOn: vpc}
- {resource: instance, dependsOn: subnet-a}
# Lets a replacement be created before its predecessor is destroyed.
- {resource: new, dependsOn: old, createBeforeDestroy: true}

# Compose NopResources that report Ready after this long, instead of
# ConfigMaps. Spreads each ordering wave over a controlled interval, which is
# what makes the sequencing observable rather than instantaneous. Requires
# provider-nop.
readyAfter: 10s

# Resources the pipeline requires but doesn't compose, and edges onto them.
# Crossplane fetches these and the dependent waits until they're ready.
requires:
- name: env
  apiVersion: apiextensions.crossplane.io/v1beta1
  kind: EnvironmentConfig
  matchName: ordering-env
requiredEdges:
- {resource: instance, requirement: env}

# Drop names from desired state, to exercise ordered teardown while the XR is
# still alive.
remove: [instance]
```

Names in `sequence` and `resources` become composed resources. A name that
appears in `edges` but is composed by neither is ignored — Crossplane prunes
edges naming resources it doesn't know about, and says so in an event.

## Readiness

With `readyAfter` set, each composed resource is a `NopResource` whose Ready
condition flips after that delay, and this function reports readiness from the
resource's own condition. Without it, resources are ConfigMaps, which have no
conditions, and existence counts as ready.

That distinction matters for testing: ConfigMaps make every ordering wave
complete inside a second, so a test can only assert eventual convergence —
which passes even with ordering disabled.

## Examples

`examples/` has working manifests for each ordering case — linear chains, a
four-level graph with a diamond, required resources, create-before-destroy,
ordered teardown, and a deliberately contradictory graph. See
[examples/README.md](examples/README.md).

## Building

```shell
./build.sh                                            # build only
./build.sh index.docker.io/you/function-ordering:tag  # build and push
```

Cross-compiles `linux/amd64` and `linux/arm64` and produces a multi-arch
package. Needs Docker and the [Crossplane CLI][cli].

## Pinning

`go.mod` replaces `github.com/crossplane/crossplane/v2` with a commit from the
prototype branch, because the `Dependencies` proto messages exist nowhere else.
If those messages change, rebuild and republish — a stale image fails tests
confusingly. Drop the replace once the field lands upstream.

[design]: https://github.com/stevendborrelli/crossplane/blob/composed-resource-ordering/design/design-doc-composed-resource-ordering.md
[branch]: https://github.com/stevendborrelli/crossplane/tree/composed-resource-ordering
[cli]: https://docs.crossplane.io/latest/cli/
