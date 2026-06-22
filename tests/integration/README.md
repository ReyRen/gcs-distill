# GCS-Distill Integration Tests

This directory contains repository-local integration checks for the distillation
control plane. It does not build or run the EasyDistill runtime image. Runtime
image construction belongs to the EasyDistill/image pipeline, while
`gcs-distill` only submits the configured `executor.runtime_image` reference to
`gcs-v2`.

## Scripts

### `test_e2e_workflow.sh`

Creates a temporary distillation run workspace and verifies the files that
`gcs-distill` must prepare or consume:

- workspace directory layout
- seed dataset copy
- EasyDistill config files
- simulated teacher output
- log persistence
- simulated student checkpoint output

Run it from the repository root:

```bash
./tests/integration/test_e2e_workflow.sh
```

or through Make:

```bash
make test-e2e
make test-integration
```

## Test Data

`sample_seed_data.json` contains five sample seed instructions used by the
workspace smoke test.

## Boundary

Docker, GPU pass-through, and EasyDistill CLI smoke tests are intentionally not
kept here. Those checks should live with the runtime image build/release
pipeline, because worker nodes only need to pull an already published image.
