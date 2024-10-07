package veilvault

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func Encode(dirPath, imagePath string, password string, excludes []string) error {
	// Step 1: Create a buffer to hold the ZIP data
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	// Step 2: Walk through the directory and zip the files
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Use filepath.Rel to get the relative path, ensuring that we're not exceeding slice bounds
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Check if the file or directory is in the excludes list
		if len(excludes) > 0 && shouldExclude(relPath, excludes) {
			fmt.Println(" › excluded", relPath, excludes, len(excludes))
			return nil // Skip this file/directory by returning nil
		} else {
			fmt.Println(" › included", relPath)
		}

		// Create a zip header for each file or directory
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header to zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a file, write the file content to the zip
		if !info.IsDir() {
			fileContent, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, err = writer.Write(fileContent)
			if err != nil {
				return err
			}
		}
		return nil
	})

	// Step 3: Close the zip writer
	if err := zipWriter.Close(); err != nil {
		return err
	}

	// Step 4: Convert the zipped bytes into an image
	zipBytes := buf.Bytes()
	img, err := bytesToImage("any-file.zip", zipBytes, 256)
	if err != nil {
		return fmt.Errorf("could not convert file to image: %v", err)
	}

	// Create the output PNG image file.
	outFile, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("could not create output image file: %v", err)
	}
	defer outFile.Close()

	// Encode the image as PNG.
	err = png.Encode(outFile, img)
	if err != nil {
		return fmt.Errorf("could not encode image to PNG: %v", err)
	}

	return nil
}

var re = regexp.MustCompile(`[^0-9]+`)

func Decode(imagePath, outputDir string, password string) error {
	// Step 1: Open the image
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return err
	}

	// Step 3: Extract metadata from the first row of the image
	var metaBytes []byte
	imgWidth := img.Bounds().Max.X
	for x := 0; x < imgWidth; x++ {
		r, g, b, _ := img.At(x, 0).RGBA()
		metaBytes = append(metaBytes, byte(r>>8), byte(g>>8), byte(b>>8))
	}

	// Extract file name and file size
	metaParts := strings.Split(string(metaBytes), "|")
	fileName := metaParts[0]
	fileSize, err := strconv.Atoi(re.ReplaceAllString(metaParts[1], ""))
	if err != nil {
		return err
	}

	if false {
		fmt.Println("Decode() › ", map[string]any{
			"metaBytes": string(metaBytes),
			"fileName":  fileName,
			"fileSize":  fileSize,
			"imgWidth":  imgWidth,
			"imgHeight": img.Bounds().Max.Y,
		})
	}

	// Step 4: Extract file data from the image pixels
	var zipBytes []byte
	bounds := img.Bounds()

scan:
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			if a < 255 && y != 0 {
				if true {
					break scan
				}
			}

			// Write the R, G, B values back as bytes (converted to 8-bit values)
			zipBytes = append(zipBytes, byte(r>>8))

			// Check if we reached the last data byte before writing G and B values
			if (x+y*bounds.Max.X)*3+1 < bounds.Max.X*bounds.Max.Y*3 {
				zipBytes = append(zipBytes, byte(g>>8))
			}

			if (x+y*bounds.Max.X)*3+2 < bounds.Max.X*bounds.Max.Y*3 {
				zipBytes = append(zipBytes, byte(b>>8))
			}
		}
	}

	// Step 3: Unzip the byte array to recover original files
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		panic(err)
		// return err
	}

	// Step 4: Restore the directory structure
	for _, f := range zipReader.File {
		// Create the directory structure as needed
		fPath := filepath.Join(outputDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fPath, os.ModePerm)
		} else {
			// Extract file
			outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func bytesToImage(fileName string, fileBytes []byte, imgWidth int) (*image.RGBA, error) {
	fileSize := len(fileBytes)
	length := len(fileBytes)
	imgHeight := length / imgWidth
	if len(fileBytes)%imgWidth != 0 {
		imgHeight++
	}

	// Create the image: +1 to imgHeight for metadata row.
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight+1))

	// Encode metadata (file name, size) in the first row of the image.
	meta := fmt.Sprintf("%s|%d", fileName, fileSize)
	metaBytes := []byte(meta)
	for i := 0; i < len(metaBytes); i += 3 {
		x := i / 3
		r, g, b := metaBytes[i], byte(0), byte(0)
		if i+1 < len(metaBytes) {
			g = metaBytes[i+1]
		}
		if i+2 < len(metaBytes) {
			b = metaBytes[i+2]
		}
		img.Set(x, 0, color.RGBA{r, g, b, 255})
	}

	// Fill the remaining image with file data.
	index, stopX, stopY := 0, 0, 0
	for y := 1; y < imgHeight+1; y++ { // Start from 1 to skip metadata row.
		for x := 0; x < imgWidth; x++ {
			if index < length {
				r, g, b, a := fileBytes[index], byte(0), byte(0), byte(255)
				if index+1 < length {
					g = fileBytes[index+1]
				} else {
					a = a - 1
				}

				if index+2 < length {
					b = fileBytes[index+2]
				} else {
					a = a - 2
				}

				img.Set(x, y, color.RGBA{r, g, b, a})
				index += 3
			} else {
				if stopX == stopY && stopX == 0 {
					stopX = x
					stopY = y
				}

				img.Set(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}

	if false {
		fmt.Println("bytesToImage() › ", map[string]any{
			"metaBytes": string(metaBytes),
			"fileName":  fileName,
			"fileSize":  fileSize,
			"imgWidth":  imgWidth,
			"imgHeight": imgHeight + 1,
			"index":     index,
			"stopX":     stopX,
			"stopY":     stopY,
		})
	}

	return img, nil
}

// Function to check if a file or directory should be excluded
func shouldExclude(path string, excludes []string) bool {
	for _, exclude := range excludes {
		// Compare against the file path, excluding matching files or directories
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}
	return false
}
