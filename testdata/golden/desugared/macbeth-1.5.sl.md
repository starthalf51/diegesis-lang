```sl
module macbeth.act1
import macbeth.cast
scene 1.5 at inverness
  when after 1.3 by 1d..3d
  enter lady_macbeth
  lady_macbeth reads macbeth_letter
  lady_macbeth thinks: "Yet do I fear thy nature; / It is too full o' the milk of human kindness"
  render
    pov lady_macbeth close
    omit letter.full_text
  exit all
```

In the castle at Inverness she reads the letter twice, the second time
aloud, and only part of it.
