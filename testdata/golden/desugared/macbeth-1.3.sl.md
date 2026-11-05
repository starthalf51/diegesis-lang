```sl
module macbeth.act1
import macbeth.cast
scene 1.3 at heath
  when after 1.2 by 0d..1d
  enter macbeth, banquo
  enter witches
  beat prophecy:
    witch1 hails macbeth: holds(thane_of_glamis, macbeth)
    witch2 hails macbeth: holds(thane_of_cawdor, macbeth)   # 참이다. 마녀는 oracle이다
    witch3 hails macbeth: king_hereafter "that shalt be king hereafter"
    witches tells banquo: banquo_line "Thou shalt get kings, though thou be none"
  setup banquo_kings promise: banquo_line
```

<!-- beat:prophecy -->

Upon the blasted heath the three shapes rose out of the fog and hailed him
by names he had not yet earned.

```sl
  #! allow belief-contradiction: "학계에서 논쟁 중이다. 1.2에서 맥베스는 코더가 도운 노르웨이군과 싸웠다. 그런데도 1.3에서 그가 살아 있고 번창한다고 말한다. 앵거스는 코더의 반역이 '숨은 도움'이었다고 해명한다."
  macbeth says: holds(thane_of_cawdor, cawdor) "the Thane of Cawdor lives, / A prosperous gentleman"
  exit witches
  enter ross, angus
  beat ross_news:
    ross tells macbeth, banquo: holds(thane_of_cawdor, macbeth) "He bade me, from him, call thee Thane of Cawdor"
    angus tells macbeth, banquo: cawdor_traitor "treasons capital, confess'd and proved"
  macbeth aside: "Two truths are told"
  macbeth believes king_hereafter because @1.3.ross_news
  macbeth thinks: "why do I yield to that suggestion / Whose horrid image doth unfix my hair"
  banquo aside: "oftentimes, to win us to our harm, / The instruments of darkness tell us truths"
  render
    pov macbeth close
    pace slow
    tension 0.8
    focus witches, macbeth.face
  exit all
```

<!-- beat:ross_news -->

Ross brings the king's word, and the second prophecy is true before the
first one has cooled.
