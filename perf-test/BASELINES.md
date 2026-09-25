# Performance baselines

This directory deliberately does **not** commit machine-specific benchmark result
trees. Those live under `perf-test/results/` locally or as GitHub Actions
artifacts.

Use the same profile on the same machine when comparing optimizations.

## Reference profiles

```text
smoke   8186 px   60 -> 30 FPS    29 universes   10 s
perf    8186 px  120 -> 60 FPS    29 universes   60 s
soak    8186 px   60 -> 30 FPS    29 universes  900 s default
stress 32768 px  120 ->120 FPS   193 universes   60 s
```

## Commit-triggered CI runs

A normal relevant push runs `smoke`.

A commit message can request another profile:

```text
[perf=perf]
[perf=stress]
[perf=soak] [seconds=120]
```

GitHub Actions uploads a complete result artifact for Windows and Linux.

## Baseline discipline

For an optimization:

1. run the chosen profile and keep its `summary.json`;
2. make one focused change;
3. run the exact same profile again on the same machine;
4. compare with `compare-results.mjs`;
5. reject correctness regressions even when raw throughput improves.

Target-machine baselines should eventually be captured on the actual Windows PC
driving the LED installation because cloud CI hardware varies between runs.
