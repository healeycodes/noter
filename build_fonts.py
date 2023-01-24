import json
import os
import glob
import shutil

store_file = "fonts/dist/fonts.store"
store_file_map = "fonts/dist/fonts.json"

# clear up fonts/dist
shutil.rmtree("fonts/dist")
os.mkdir("fonts/dist")

fonts = {}  # hex: [offset, size]
offset = 0

with open(store_file, "ab") as store_file_f:
    for png_path in glob.glob("fonts/**/*.png", recursive=True):
        name = png_path.split("/")[-1][:4]
        with open(png_path, "rb") as png_path_f:
            size = 0
            while byte := png_path_f.read(1):
                size += 1
                store_file_f.write(byte)
            fonts[f'U+{name}'] = [offset, size]
            offset += size

with open(store_file_map, "w") as f:
    json.dump(fonts, f)

print("fonts built!")
