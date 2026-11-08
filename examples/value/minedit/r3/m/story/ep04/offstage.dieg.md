---
module: yeoul.cast
mode: authored
---

# 여울섬 — 4화 무대 밖 사건

```sl
event taeo_inquiry "서윤에게서 도경태의 병원 알리바이를 전해 들은 태오가 육지 병원에 입원 기록을 알아본다"
  when after s8 by 0h..12h
  when before s9
  lasts 10m..1h
  seoyun says taeo: hospital_alibi
  taeo does call_hospital
  taeo believes taeo_checked
  taeo reads hospital_reply: hospital_no_record
  taeo believes not hospital_alibi
```
