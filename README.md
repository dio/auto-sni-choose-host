# auto-sni-choose-host

Minimal dynamic-module cluster example for the `auto_host_sni` hostname ABI patch.

Patch/prototype context:
https://gist.github.com/dio/965d1e555909c02013ca882a2b3caa78

The module registers a cluster named `auto-sni-choose-host`. At cluster init it
adds two HTTPS hosts with both a concrete socket address and a logical hostname.
At request time `ChooseHost` reads `x-target-host` and returns the matching host.

This is intentionally smaller than Plum: no product config model, no async body
handoff, and no upstream HTTP filters.

## Run

```sh
make run
```

`make run` builds `.bin/libauto_sni_choose_host.so`, renders `.bin/envoy.yaml`,
and downloads the matching patched Envoy binary from
`dio/envoy-builder` release
`envoy-0d6e3c60-auto-host-sni-bounded-sni-session-cache` when `.bin/envoy` is
not present.

Use a local Envoy binary instead:

```sh
make run ENVOY_BIN=/path/to/patched/envoy
```

Override the trust bundle if your platform uses a different path:

```sh
make run TRUSTED_CA=/etc/ssl/certs/ca-certificates.crt
```

In another shell:

```sh
make request-example
make request-iana
```

The important config is in `config/envoy.yaml.in`:

- `auto_host_sni: true`
- `auto_sni_san_validation: true`
- one shared `UpstreamTlsContext`
- a dynamic-module cluster whose hosts carry `hostname`
- DNS names are resolved before `AddHosts`; the original hostname is still
  passed separately for SNI.

Without the patched hostname ABI, runtime-added hosts do not provide the logical
hostname that `auto_host_sni` needs.
