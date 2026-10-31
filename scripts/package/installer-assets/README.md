# Installer branding assets

`WizardImage.bmp` (164x314) and `WizardSmallImage.bmp` (55x58) are the
branded images `runyx.iss` points `WizardImageFile`/
`WizardSmallImageFile` at -- Inno Setup requires 24-bit BMP, not
PNG/SVG, so these are pre-rendered rather than generated at build
time.

Regenerated from `assets/mascot/svg/runyx-mascot-idle-dark.svg` (on
`feat/runyx-code`; this branch doesn't carry `assets/mascot/`) onto
Runyx's own dark navy (`#1B2233`, the mascot's own background color)
with `rsvg-convert` + ImageMagick:

```sh
rsvg-convert -w 300 -h 300 runyx-mascot-idle-dark.svg -o mascot-300.png
convert -size 164x314 xc:"#1B2233" wizard-bg.png
convert mascot-300.png -filter point -resize 120x120 mascot-120.png
convert wizard-bg.png mascot-120.png -gravity center -geometry +0-40 -composite wizard-large-tmp.png
convert wizard-large-tmp.png -gravity south -fill "#F4EFE6" -pointsize 22 \
  -font DejaVu-Sans-Bold -annotate +0+30 "Runyx" wizard-large.png
convert wizard-large.png -background "#1B2233" -flatten BMP3:WizardImage.bmp

convert -size 55x58 xc:"#1B2233" small-bg.png
rsvg-convert -w 48 -h 48 runyx-mascot-idle-dark.svg -o mascot-48.png
convert mascot-48.png -filter point -resize 44x44 mascot-44.png
convert small-bg.png mascot-44.png -gravity center -composite \
  -background "#1B2233" -flatten BMP3:WizardSmallImage.bmp
```

`-filter point` on the resize steps keeps the pixel-art mascot crisp
(no blur/antialiasing blending its hard pixel edges) instead of the
smooth default resampling filter, which would turn it to mush at this
size.
