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
lasts 10m..20m
enter seoyun, minjae, taeil
minjae tells seoyun: minjae_leaves
seoyun asks minjae
minjae says seoyun: malsun_dementia
minjae tells taeil: malsun_dementia
seoyun thinks: minjae_leaves
minjae says seoyun: vote_in_12
minjae does board_ferry
seoyun sees minjae
exit minjae
exit seoyun, taeil
```
