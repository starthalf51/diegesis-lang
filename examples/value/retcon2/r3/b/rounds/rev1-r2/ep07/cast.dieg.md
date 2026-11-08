---
module: yeoul.cast
mode: authored
---

# 여울섬 — 7화 추가 선언

```sl
fact chart_search "해솔은 보건지소 창고에서 삼십 년 전의 옛 진료부를 찾아낸다" true
fact gyeongtae_treated "해진호가 가라앉은 이튿날 아침 도경태는 여울섬 보건지소에서 오른손에 깊게 베인 상처를 꿰맸다" true
fact chart_signed "그 진료 기록 줄 끝에는 도경태가 직접 쓴 이름과 서명이 남아 있다" true
fact alibi_broken "진료부는 서윤이 등대에서 이미 알아챈 대로 도경태가 그 밤 육지 병원이 아니라 섬에 있었다는 것을 문서로 못 박는다" true
fact haesol_keeps_quiet "해솔은 진료부를 출장소에 옮겨 보관하고 당분간 아무에게도 알리지 않기로 한다" true
fact father_scar "도경태의 오른손 손바닥에는 오래된 흉터가 길게 나 있다" true
fact malsun_bandage "해진호가 가라앉은 이튿날 아침 도경태는 피 묻은 천을 손에 감고 선착장에 서 있었다" true
fact seoyun_resolve "서윤은 오늘 밤 할머니가 잠들면 장롱 위 양철 상자를 열어 보기로 마음먹는다" true

artifact clinic_chart "the old clinic register"
  name ko "보건지소 옛 진료부"
  contains gyeongtae_treated, chart_signed

haesol believes chart_search
haesol believes haesol_keeps_quiet
haeun believes father_scar
malsun believes malsun_bandage
```
