| Test | Fälle | Abweichungen Fast (float64) | Abweichungen Precise (16.16) | Beispiel |
|---|---:|---:|---:|---|
| scale8 | 65536 | 0 | 0 |  |
| qadd8 | 65536 | 0 | 0 |  |
| qsub8 | 65536 | 0 | 0 |  |
| sin8 | 256 | 0 | 0 |  |
| sin16 | 65536 | 0 | 0 |  |
| random16 x100000 | 100000 | 99995 | 0 | fast draw#3 = 34660, want 34661 |
| random8 x100000 | 100000 | 99604 | 0 | fast draw#3 = 235, want 236 |
| beat8 | 8154 | 0 | 0 |  |
| beatsin8 | 8154 | 0 | 0 |  |
| beatsin8 naive wave()-idiom (float) | 5418 | 5319 | – | max abs error 209 levels |

| Funktion | JS-Referenz FNV | natives FastLED FNV | identisch |
|---|---|---|---|
| scale8 | fb7ba54f375bd367 | fb7ba54f375bd367 | ja |
| qadd8 | 0a6a903eb621bf83 | 0a6a903eb621bf83 | ja |
| qsub8 | d962d0ae03675183 | d962d0ae03675183 | ja |
| sin8 | 0f344a003df5aefd | 0f344a003df5aefd | ja |
| sin16 | 7daf363a4f73fa03 | 7daf363a4f73fa03 | ja |
| random16 | a630a3fe8841fda0 | a630a3fe8841fda0 | ja |
| random8 | 753e4368a078c6be | 753e4368a078c6be | ja |
| beat8 | 287facef88bf6755 | 287facef88bf6755 | ja |
| beatsin8 | 939d0376aed91010 | 939d0376aed91010 | ja |
