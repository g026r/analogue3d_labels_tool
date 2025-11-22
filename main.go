package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/disintegration/imaging"
)

// Image is a simple struct used to store the custom images being added
type Image struct {
	// Filepath should correspond to the path to the file, while the actual filename should be equal to the signature
	Filepath string
	// Signature is the cartridge signature. It can be found via the library or by running a CRC32 on the first 8KiB of a
	// native encoding (big endian) .z64 ROM file.
	Signature uint32
}

const (
	height = 86
	width  = 74
	// imgPadding is the size in bytes of padding added to the end of every image entry to make it the correct size
	imgPadding = 0x90
	entrySize  = (height * width * 4) + imgPadding

	// indexStart is the location in the file where the index of cartridge signatures begins
	indexStart = 0x100
	// indexEOF is the word that indicates there are no more cartridges in the index
	indexEOF uint32 = 0xFFFFFFFF
	// imgsStart is the location in the file where the first image begins
	imgsStart = 0x4100

	// header is not currently used since I'm modifying the file in place. But it's here for potential future use
	header = "\aAnalogue-Co\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000Analogue-3D.labels\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0002\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000\u0000"
)

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		log.Fatalf("usage: %s {labels.db} {image files}", os.Args[0])
	}
	labelsDB, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	customImgs, err := generateListFromArgs(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile(labelsDB, os.O_RDWR, 777)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	sigs := make([]uint32, 0)
	if _, err := f.Seek(indexStart, 0); err != nil {
		log.Fatal(err)
	}

	// 32 bit words, so imgsStart - indexStart must be divided by 4 to give the number of possible entries
	for i := 0; i < (imgsStart-indexStart)/4; i++ {
		var sig uint32
		if err := binary.Read(f, binary.LittleEndian, &sig); err != nil {
			log.Fatal(err)
		}
		if sig == indexEOF {
			break
		}
		sigs = append(sigs, sig)
	}

	if _, err := f.Seek(imgsStart, 0); err != nil {
		log.Fatal(err)
	}
	imgs := make([][]byte, 0)
	for range sigs {
		px := make([]byte, entrySize)
		if err := binary.Read(f, binary.BigEndian, &px); err != nil {
			log.Fatal(err)
		}
		imgs = append(imgs, px)
	}

	sigs, imgs = buildNewDB(sigs, imgs, customImgs)

	// Write out the new values in place
	log.Printf("Writing %d images to %s", len(imgs), labelsDB)
	if _, err := f.Seek(indexStart, 0); err != nil {
		log.Fatal(err)
	}

	if err := binary.Write(f, binary.LittleEndian, sigs); err != nil {
		log.Fatal("sigs", err)
	}
	if err := binary.Write(f, binary.LittleEndian, indexEOF); err != nil {
		log.Fatal("eof", err)
	}
	if _, err := f.Seek(imgsStart, 0); err != nil {
		log.Fatal(err)
	}
	for i := range imgs {
		if err := binary.Write(f, binary.BigEndian, imgs[i]); err != nil {
			log.Fatal(i, err)
		}
	}
}

// generateListFromArgs takes the command line list of args & turns them into a slice of Image objects. It does not check
// if the files exist.
func generateListFromArgs(args []string) ([]Image, error) {
	imgs := make([]Image, 0)
	for _, arg := range args {
		file, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		img := Image{
			Filepath: file,
		}

		sig, err := HexStringTransform(strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg)))
		if err != nil {
			return nil, err
		}
		img.Signature = sig
		imgs = append(imgs, img)
	}

	return imgs, nil
}

// buildNewDB takes the old sigs & images, as well as the new custom images to add, and creates the correct set of arrays
// that can then be written back to the labels.db file
func buildNewDB(sigs []uint32, imgs [][]byte, customImgs []Image) ([]uint32, [][]byte) {
	slices.SortFunc(customImgs, func(a, b Image) int {
		if a.Signature < b.Signature {
			return -1
		} else if a.Signature > b.Signature {
			return 1
		}
		return 0
	})

	newSigs := make([]uint32, 0)
	newImgs := make([][]byte, 0)
	i := 0
	j := 0

	for i < len(sigs) && j < len(customImgs) {
		if sigs[i] < customImgs[j].Signature {
			newSigs = append(newSigs, sigs[i])
			newImgs = append(newImgs, imgs[i])
			i++
		} else if sigs[i] > customImgs[j].Signature {
			newSigs = append(newSigs, customImgs[j].Signature)
			b, err := loadImage(customImgs[j].Filepath)
			if err != nil {
				log.Fatal(err)
			}
			newImgs = append(newImgs, b)
			j++
		} else { // If the signature is equal, replace the old image with the new one
			newSigs = append(newSigs, customImgs[j].Signature)
			b, err := loadImage(customImgs[j].Filepath)
			if err != nil {
				log.Fatal(err)
			}
			newImgs = append(newImgs, b)
			i++
			j++
		}
	}

	if i < len(sigs) {
		newSigs = append(newSigs, sigs[i:]...)
		newImgs = append(newImgs, imgs[i:]...)
	} else {
		for j = j; j < len(customImgs); j++ {
			newSigs = append(newSigs, customImgs[j].Signature)
			b, err := loadImage(customImgs[j].Filepath)
			if err != nil {
				log.Fatal(err)
			}
			newImgs = append(newImgs, b)
		}
	}

	return newSigs, newImgs
}

// loadImage takes a filename, loads the file from disk using getImg, resizes it to the correct dimensions, and returns a byte array
// of the BGRA representation of the image
func loadImage(filename string) ([]byte, error) {
	log.Printf("Loading %s\n", filename)
	i, err := getImg(filename)
	if err != nil {
		return nil, err
	}
	img := imaging.Resize(i, width, height, imaging.Lanczos)

	bgra := make([]byte, 0)
	// Since it's one row at a time, outer loop should be Y & inner loop should be X
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.NRGBAAt(x, y)
			bgra = append(bgra, c.B, c.G, c.R, c.A)
		}
	}

	// Add the 144 bytes of padding
	for i := 0; i < imgPadding; i++ {
		bgra = append(bgra, 0xFF)
	}

	return bgra, nil
}

// getImg loads an image from disk. I copied this from an old project and can't recall why I'm using it rather than
// imaging.Open. I think image.Decode might handle a greater number of file formats?
func getImg(src string) (img image.Image, err error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	i, _, err := image.Decode(f)
	return i, err
}

// HexStringTransform takes a string, validates that it is a 32 bit hex string, and returns the uint32 representation of it
// The input string may or may not be prefixed with `0x` and any leading or trailing spaces are removed.
// If a blank string is passed, 0 is returned
func HexStringTransform(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// take care of the many different ways a user might input this
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return 0, fmt.Errorf("invalid string provided: %s", s)
	}

	// String should be exactly 32 bits. We can pad it out if too short, but can't handle too long.
	if len(s) > 8 {
		return 0, fmt.Errorf("hex string too long: %s", s)
	} else if len(s) < 8 {
		s = fmt.Sprintf("%08s", s) // binary.BigEndian.Uint32 fails if not padded out to 32 bits
	}

	h, err := hex.DecodeString(s)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(h), nil
}
