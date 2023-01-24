rm -rf ./ark-pixel-font/
git clone https://github.com/TakWolf/ark-pixel-font
cd ark-pixel-font
git checkout 04cca2c0ca25c8d4c3877903072882116bdcf3ca
cd ..
mv ark-pixel-font/assets/glyphs/12/monospaced/ fonts/monospaced
rm -rf ./ark-pixel-font/
python3 build_fonts.py
