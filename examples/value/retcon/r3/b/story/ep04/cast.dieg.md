---
module: yeoul.cast
mode: authored
---

# 여울섬 — 4화 추가 선언

```sl
fact climb_before "서윤과 태오는 등대가 철거되기 전에 꼭대기까지 한 번 올라가 보기로 한다" true
fact stairs_old "등대 나선 계단의 못은 하나같이 붉게 녹슬어 있다" true
fact nail_new "열세 번째 계단에만 녹 하나 없는 새 못이 박혀 있다" true
fact nail_odd "아무도 쓰지 않는 등대 계단에 누가 왜 새 못을 박았는지 알 수 없다" true
fact step_mark "열세 번째 계단 판자 가장자리에는 한 번 들어 올렸다가 다시 덮은 듯한 틈이 나 있다" true
fact lens_chipped "꼭대기 등실의 렌즈 한 귀퉁이가 깨져 떨어져 나가 있다" true
fact lens_grooves "등대 렌즈 유리에는 가는 홈이 층층이 둥글게 패어 있다" true
fact shard_matches "정수아가 주운 유리 조각은 깨진 등대 렌즈 귀퉁이에서 떨어져 나온 것이다" true
fact taeo_checked "윤태오는 보건지소 일로 육지 병원에 연락한 김에 해진호 사고 무렵 도경태의 입원 기록을 알아보았다" true
fact hospital_no_record "육지 병원에는 해진호가 가라앉던 무렵 도경태가 입원했던 기록이 전혀 없다" true
fact keep_secret_alibi "서윤과 태오는 확실한 증거를 손에 쥘 때까지 이 일을 아무에게도 말하지 않기로 한다" true

artifact hospital_reply "the mainland hospital's reply"
  name ko "육지 병원 회신 문서"
  contains hospital_no_record

seoyun believes climb_before
taeo believes climb_before
```
