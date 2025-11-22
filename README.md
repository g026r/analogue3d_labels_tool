# analogue3d_labels_tool

Quick tool to insert images into an Analogue 3D's labels.db

## Usage:

If using the compiled version:
`a3dlabels <path to labels.db> <path to image to add>`

### Important Notes:

1. This tool updates the labels.db file in place. Make a backup of your original file before running it.
2. While common image formats are supported and images will be resized to the correct dimensions, aspect ratios are not
   respected. The final image is 74x86, so it should have that aspect ratio to start with.
3. Images **_MUST_** have a filename that corresponds to the cartridge signature. e.g. If you are adding a cartridge
   whose signature is 3274BDAF, then the file should be named 3274BDAF.png (or 3274BDAF.jpg, or 3274BDAF.bmp, &amp;c.)