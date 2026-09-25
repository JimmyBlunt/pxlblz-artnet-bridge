# Virtual PXLBLZ integration test

This test environment exercises the **real** `externalPixelOutput.ts` adapter,
not the Go dummy frame sender.

It verifies two paths:

1. exact TypeScript adapter → real RFC6455 WebSocket → real router → UDP Art-Net
   listener with the 8000-pixel / 48-universe loopback config;
2. exact TypeScript adapter → real router using the production 8186-pixel
   BACK_PANEL route in `--dry-run`, where 29 universes/frame must produce
   870 packets/s at 30 FPS.

The test also runs an adapter self-test for:

- Float [0,1] → RGB888 conversion and clamping;
- invalid frame-size rejection;
- browser WebSocket backpressure drop behavior;
- disabled `?pxout` no-op behavior.

## Requirements

- Node.js 22+
- TypeScript `tsc` in PATH
- Go 1.23+

No browser, controller, LED hardware, or external network connection is needed.

## Run

From repository root:

```bash
node pxlblz-integration/virtual-test/run-virtual-e2e.mjs
```

Successful completion ends with:

```text
VIRTUAL_E2E_A_PASS ...
VIRTUAL_E2E_B_PASS ...
ALL_VIRTUAL_TESTS_PASS
```

The generated `build/` and `.tmp/` directories are disposable test output.
