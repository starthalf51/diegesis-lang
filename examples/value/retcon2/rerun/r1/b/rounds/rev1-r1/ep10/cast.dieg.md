---
module: yeoul.cast
mode: authored
---

# 여울섬 — 10화 추가 선언

```sl
fact gyeongtae_came_secretly "그날 밤 도경태는 육지 병원에 있지 않고 몰래 배를 타고 섬에 들어와 있었다" true
fact quarrel_in_lighthouse "그 밤 등실에서 백영호와 도경태는 불을 끄느냐 마느냐를 두고 멱살을 잡고 다투었다" true
fact lens_broke_alone "다툼 끝에 도경태가 램프 덮개를 억지로 닫아 불을 끄고 내려간 뒤, 혼자 남은 백영호가 불을 다시 켜려다 어둠 속에서 램프에 부딪혀 렌즈를 깨뜨렸다" true
fact gyeongtae_threat "도경태는 불이 꺼진 일을 등대지기 탓으로 돌리겠다고 을러 백영호의 입을 막았다" true
fact light_died_tide "등대 불이 죽은 것은 열한 물때 들물이 막 돌던 시각이었다" true
fact yeongho_testify "백영호는 투표 전 마을 회의에서 그 밤 일을 증언하겠다고 한다" true
fact statement_taken "해솔은 백영호의 진술을 받아 적어 진료부, 파편 꾸러미와 함께 출장소에 보관하기로 한다" true
fact malsun_saw_dark "그 밤 오말순은 집 창으로 등대 불이 꺼지는 것을 보았다" true
fact song_is_clock "옛 노래의 '열한 물때에 불이 잔다'는 그 밤 등대 불이 죽은 시각을 가리킨다" true

artifact yeongho_statement "Baek Yeongho's statement"
  name ko "백영호 진술서"
  contains gyeongtae_came_secretly, quarrel_in_lighthouse, lens_broke_alone, light_died_tide

yeongho believes gyeongtae_came_secretly
yeongho believes quarrel_in_lighthouse
yeongho believes lens_broke_alone
yeongho believes gyeongtae_threat
yeongho believes light_died_tide
yeongho believes yeongho_testify
haesol believes statement_taken
malsun believes malsun_saw_dark
malsun believes song_is_clock
```
