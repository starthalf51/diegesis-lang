---
module: yeoulseom.scenes
import: [yeoulseom.cast]
scene: "s7b"
location: harbor
when: [after s7 by 0d..1d, before s8]
pov: seoyun
render:
  pace: medium
  tension: 0.5
mode: authored
---

```sl
lasts 20m..40m
enter seoyun, minjae, taeil
minjae tells seoyun: minjae_leaves
seoyun asks minjae
minjae says seoyun: malsun_dementia
minjae tells taeil: malsun_dementia
minjae does board_ferry
exit minjae
attr minjae.status = gone
taeil says seoyun: vote_in_12
seoyun asks taeil
taeil says seoyun: lighthouse_closed
seoyun thinks: song_lyric
exit seoyun, taeil
```
