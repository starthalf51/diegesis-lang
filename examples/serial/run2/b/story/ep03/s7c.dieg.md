---
module: yeoulseom.scenes
import: [yeoulseom.cast]
scene: "s7c"
location: harbor
when: [after s7b by 0m..10m, before s8]
pov: seoyun
render:
  pace: medium
  tension: 0.5
mode: authored
---

```sl
lasts 15m..30m
attr minjae.status = gone
enter seoyun, taeil
taeil says seoyun: vote_in_12
seoyun asks taeil
taeil says seoyun: lighthouse_closed
taeil says seoyun: official_cause
seoyun says taeil: song_lyric
taeil asks seoyun
seoyun says taeil: father_died
seoyun thinks: song_lyric
exit seoyun, taeil
```
