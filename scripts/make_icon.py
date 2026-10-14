"""Create the PhotoKeep desktop icon (Pillow is only needed to regenerate it)."""
from PIL import Image, ImageDraw
from pathlib import Path

scale = 2
size = 1024 * scale
im = Image.new('RGBA', (size, size), (0, 0, 0, 0))
d = ImageDraw.Draw(im)
S = lambda n: n * scale
d.rectangle((S(64), S(64), S(960), S(960)), fill=(39, 106, 73, 255))
d.rectangle((S(220), S(275), S(805), S(755)), outline=(255, 255, 255, 255), width=S(42))
d.ellipse((S(625), S(360), S(695), S(430)), fill=(255, 255, 255, 255))
d.polygon([(S(243), S(704)), (S(390), S(508)), (S(507), S(626)), (S(595), S(530)), (S(783), S(714))], fill=(255, 255, 255, 255))
d.rectangle((S(474), S(760), S(802), S(825)), fill=(255, 255, 255, 255))
im = im.resize((1024, 1024), Image.Resampling.LANCZOS)
Path('build').mkdir(exist_ok=True)
im.save('build/appicon.png')
