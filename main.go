package main

import (
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// transliteration table for Ukrainian and Russian Cyrillic characters
var cyrillicToLatin = map[rune]string{
	// Ukrainian Cyrillic to Latin
	'А': "A", 'Б': "B", 'В': "V", 'Г': "H", 'Ґ': "G", 'Д': "D", 'Е': "E", 'Є': "Ye", 'Ж': "Zh",
	'З': "Z", 'И': "Y", 'І': "I", 'Ї': "Yi", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M", 'Н': "N",
	'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U", 'Ф': "F", 'Х': "Kh", 'Ц': "Ts",
	'Ч': "Ch", 'Ш': "Sh", 'Щ': "Shch", 'Ю': "Yu", 'Я': "Ya", 'Ь': "",

	// Lowercase Ukrainian Cyrillic
	'а': "a", 'б': "b", 'в': "v", 'г': "h", 'ґ': "g", 'д': "d", 'е': "e", 'є': "ye", 'ж': "zh",
	'з': "z", 'и': "y", 'і': "i", 'ї': "yi", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "shch", 'ю': "yu", 'я': "ya", 'ь': "",

	// Russian Cyrillic (to provide additional support)
	'Ё': "E", 'Ы': "Y", 'Э': "E", 'ё': "e", 'ы': "y", 'э': "e",
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Google Spreadsheet Image Downloader")

	// Input fields with labels
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Enter Google Spreadsheet URL")
	urlLabel := widget.NewLabel("Spreadsheet URL:")

	hostnameEntry := widget.NewEntry()
	hostnameEntry.SetPlaceHolder("Hostname (e.g., https://site.com.ua)")
	hostnameEntry.SetText("https://site.com.ua") // Default value
	hostnameLabel := widget.NewLabel("Images Default Hostname:")

	imagedirEntry := widget.NewEntry()
	imagedirEntry.SetPlaceHolder("Image Directory (e.g., /content/uploads/images/)")
	imagedirEntry.SetText("/content/uploads/images/") // Default value
	imagedirLabel := widget.NewLabel("Directory to Download Files:")

	outputFileEntry := widget.NewEntry()
	outputFileEntry.SetPlaceHolder("Output CSV File Name (e.g., output.csv)")
	outputFileEntry.SetText("output.csv") // Default value
	outputFileLabel := widget.NewLabel("File with Updated Descriptions:")

	// Separator selection
	separatorOptions := []string{"Comma (,)", "Semicolon (;)", "Tab (\\t)"}
	separatorEntry := widget.NewSelect(separatorOptions, func(value string) {
		// Handle selection change if needed
	})
	separatorEntry.SetSelected("Semicolon (;)") // Default value
	separatorLabel := widget.NewLabel("CSV Separator:")

	// Data bindings
	progressBinding := binding.NewFloat()
	statusBinding := binding.NewString()

	// Progress Bar and Status Label
	progressBar := widget.NewProgressBarWithData(progressBinding)
	statusLabel := widget.NewLabelWithData(statusBinding)

	// Initialize bindings
	progressBinding.Set(0)
	statusBinding.Set("Status: Idle")

	// Process Button
	processButton := widget.NewButton("Process Images", func() {
		spreadsheetURL := urlEntry.Text
		hostname := hostnameEntry.Text
		imagedir := imagedirEntry.Text
		outputFileName := outputFileEntry.Text
		selectedSeparator := separatorEntry.Selected

		// Collect missing fields
		var missingFields []string
		if spreadsheetURL == "" {
			missingFields = append(missingFields, "Spreadsheet URL")
		}
		if hostname == "" {
			missingFields = append(missingFields, "Hostname")
		}
		if imagedir == "" {
			missingFields = append(missingFields, "Image Directory")
		}
		if outputFileName == "" {
			missingFields = append(missingFields, "Output CSV File Name")
		}

		// Show detailed error message if any fields are missing
		if len(missingFields) > 0 {
			showError(myWindow, errors.New("Please fill in the following fields: "+strings.Join(missingFields, ", ")))
			return
		}

		// Function to start processing
		startProcessing := func() {
			go func() {
				updateStatus(statusBinding, "Fetching CSV data...")
				csvURL, err := getCSVURL(spreadsheetURL)
				if err != nil {
					showError(myWindow, err)
					updateStatus(statusBinding, "Status: Idle")
					return
				}

				records, err := fetchCSV(csvURL)
				if err != nil {
					showError(myWindow, err)
					updateStatus(statusBinding, "Status: Idle")
					return
				}

				updateStatus(statusBinding, "Processing records...")
				err = processRecords(records, hostname, imagedir, outputFileName, selectedSeparator, progressBinding, statusBinding, myWindow)
				if err != nil {
					showError(myWindow, err)
					updateStatus(statusBinding, "Status: Idle")
					return
				}

				updateStatus(statusBinding, "Status: Completed")
				showInfo(myWindow, "Images downloaded and data processed successfully.\nOutput saved to "+outputFileName)
			}()
		}

		// Function to check output file and proceed
		checkOutputFileAndProcess := func() {
			if fileExists(outputFileName) {
				dialog.ShowConfirm("File Exists",
					fmt.Sprintf("The output file '%s' already exists. Do you want to delete it and proceed?", outputFileName),
					func(confirmed bool) {
						if confirmed {
							err := os.Remove(outputFileName)
							if err != nil {
								showError(myWindow, fmt.Errorf("Failed to delete file '%s': %v", outputFileName, err))
								return
							}
							startProcessing()
						} else {
							updateStatus(statusBinding, "Operation Aborted")
							showError(myWindow, fmt.Errorf("File '%s' exists, aborting...", outputFileName))
							return
						}
					}, myWindow)
			} else {
				startProcessing()
			}
		}

		// Check if image directory exists
		imageDirPath := filepath.Join("files", imagedir)
		if dirExists(imageDirPath) {
			dialog.ShowConfirm("Directory Exists",
				fmt.Sprintf("The directory '%s' already exists. Do you want to delete it and proceed?", imageDirPath),
				func(confirmed bool) {
					if confirmed {
						err := os.RemoveAll(imageDirPath)
						if err != nil {
							showError(myWindow, fmt.Errorf("Failed to delete directory '%s': %v", imageDirPath, err))
							return
						}
						checkOutputFileAndProcess()
					} else {
						updateStatus(statusBinding, "Operation Aborted")
						showError(myWindow, fmt.Errorf("Directory '%s' exists, aborting...", imageDirPath))
						return
					}
				}, myWindow)
		} else {
			checkOutputFileAndProcess()
		}
	})

	// Organize UI Elements
	mainbox := container.New(
		layout.NewFormLayout(),
		urlLabel, urlEntry,
		hostnameLabel, hostnameEntry,
		imagedirLabel, imagedirEntry,
		outputFileLabel, outputFileEntry,
		separatorLabel, separatorEntry,
	)

	content := container.NewVBox(
		mainbox,
		processButton,
		progressBar,
		statusLabel,
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.ShowAndRun()
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// getCSVURL transforms the Google Spreadsheet URL to its CSV export URL
func getCSVURL(spreadsheetURL string) (string, error) {
	u, err := url.Parse(spreadsheetURL)
	if err != nil {
		return "", err
	}

	parts := strings.Split(u.Path, "/")
	var spreadsheetID string
	for i, part := range parts {
		if part == "d" && i+1 < len(parts) {
			spreadsheetID = parts[i+1]
			break
		}
	}
	if spreadsheetID == "" {
		return "", errors.New("Invalid Google Spreadsheet URL")
	}

	q := u.Query()
	gid := q.Get("gid")
	if gid == "" {
		if u.Fragment != "" {
			fragParts := strings.Split(u.Fragment, "=")
			if len(fragParts) == 2 && fragParts[0] == "gid" {
				gid = fragParts[1]
			}
		}
		if gid == "" {
			gid = "0"
		}
	}

	csvURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%s", spreadsheetID, gid)
	return csvURL, nil
}

// fetchCSV retrieves and parses the CSV data from the given URL
func fetchCSV(csvURL string) ([][]string, error) {
	resp, err := http.Get(csvURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to fetch CSV data: %s", resp.Status)
	}

	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

// processRecords processes the CSV data and downloads images
func processRecords(records [][]string, hostname, imagedir string, outputFileName string, selectedSeparator string, progressBinding binding.Float, statusBinding binding.String, myWindow fyne.Window) error {
	if len(records) < 2 {
		return errors.New("No data in CSV")
	}

	headers := records[0]
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[h] = i
	}

	requiredColumns := []string{"body_uk", "body_ru"}
	for _, col := range requiredColumns {
		if _, ok := headerMap[col]; !ok {
			return fmt.Errorf("Missing required column: %s", col)
		}
	}

	progressBinding.Set(0)

	var allImageLinks []string

	// Collect all image links
	imageLinkSet := make(map[string]struct{})
	for _, row := range records[1:] {
		bodyUk := row[headerMap["body_uk"]]
		bodyRu := row[headerMap["body_ru"]]

		bodyUkLinks := extractImageLinks(bodyUk)
		bodyRuLinks := extractImageLinks(bodyRu)

		for _, link := range bodyUkLinks {
			imageLinkSet[link] = struct{}{}
		}
		for _, link := range bodyRuLinks {
			imageLinkSet[link] = struct{}{}
		}
	}

	// Convert imageLinkSet to a slice
	for link := range imageLinkSet {
		allImageLinks = append(allImageLinks, link)
	}

	totalImages := len(allImageLinks)
	totalSteps := totalImages
	var stepsCompleted int
	var mu sync.Mutex

	updateStatus(statusBinding, "Downloading images...")
	// Download images
	imagePathMap := make(map[string]string)

	var wg sync.WaitGroup
	for _, link := range allImageLinks {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			newPath, err := downloadAndSaveImage(link, hostname, imagedir, myWindow)
			if err != nil {
				fmt.Printf("Error downloading image %s: %v\n", link, err)
				return
			}
			mu.Lock()
			imagePathMap[link] = newPath
			stepsCompleted++
			progress := float64(stepsCompleted) / float64(totalSteps)
			progressBinding.Set(progress)
			statusString := fmt.Sprintf(
				"Downloaded %d of %d images\nloading %s\n",
				stepsCompleted,
				totalImages,
				link,
			)
			updateStatus(statusBinding, statusString)
			mu.Unlock()
		}(link)
	}
	wg.Wait()

	// Replace image URLs in body_uk and body_ru
	for _, row := range records[1:] {
		bodyUk := row[headerMap["body_uk"]]
		bodyRu := row[headerMap["body_ru"]]

		newBodyUk := replaceImageLinks(bodyUk, imagePathMap)
		newBodyRu := replaceImageLinks(bodyRu, imagePathMap)

		row[headerMap["body_uk"]] = newBodyUk
		row[headerMap["body_ru"]] = newBodyRu

		// Update progress bar
		mu.Lock()
		stepsCompleted++
		progress := float64(stepsCompleted) / float64(totalSteps)
		progressBinding.Set(progress)
		mu.Unlock()
	}

	// Write the modified records back to a CSV file
	updateStatus(statusBinding, "Writing to output file...")
	err := writeCSV(records, outputFileName, selectedSeparator)
	if err != nil {
		return err
	}

	return nil
}

// extractImageLinks extracts image URLs from HTML content
func extractImageLinks(htmlContent string) []string {
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)
	var links []string
	for _, match := range matches {
		links = append(links, match[1])
	}
	return links
}

// replaceImageLinks replaces image URLs in HTML content with new paths
func replaceImageLinks(htmlContent string, imagePathMap map[string]string) string {
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	newContent := re.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		srcRe := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRe.FindStringSubmatch(imgTag)
		if len(srcMatch) > 1 {
			originalLink := srcMatch[1]
			if newPath, ok := imagePathMap[originalLink]; ok {
				return strings.Replace(imgTag, originalLink, newPath, 1)
			}
		}
		return imgTag
	})
	return newContent
}

// downloadAndSaveImage downloads an image from a URL and saves it to the desired path
func downloadAndSaveImage(imageURL, hostname, imagedir string, myWindow fyne.Window) (string, error) {
	// Prepare the image URL
	if strings.HasPrefix(imageURL, "//") {
		imageURL = "https:" + imageURL
	} else if !strings.HasPrefix(imageURL, "http") {
		imageURL = strings.TrimRight(hostname, "/") + "/" + strings.TrimLeft(imageURL, "/")
	}

	// Prepare the filename
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("Invalid image URL: %v", err)
	}

	// Extract the filename and make it unique by appending part of the URL path
	pathParts := strings.Split(parsedURL.Path, "/")
	uniquePart := ""
	if len(pathParts) >= 3 {
		// Use the last two directories if available
		uniquePart = strings.Join(pathParts[len(pathParts)-3:len(pathParts)-1], "_")
	} else if len(pathParts) >= 2 {
		// Use the last directory if only one directory exists
		uniquePart = pathParts[len(pathParts)-2]
	}

	filenameWithExt := filepath.Base(parsedURL.Path)
	filenameWithExt = strings.Split(filenameWithExt, "?")[0] // Remove query params
	extension := filepath.Ext(filenameWithExt)
	if extension == "" {
		extension = ".jpg" // Default to 'jpg' if no extension is present
	} else {
		// extension = extension[1:] // Remove the '.' from the extension
	}
	filename := strings.TrimSuffix(filenameWithExt, filepath.Ext(filenameWithExt))

	// Generate a SHA-1 hash of the URL to make the filename unique
	hasher := sha1.New()
	hasher.Write([]byte(imageURL))
	hash := hex.EncodeToString(hasher.Sum(nil))[:4] // Shorten the hash to 4 characters

	// Combine the unique part with the filename
	filename = hash + "-" + uniquePart + "-" + filename
	filename = transliterate(filename)
	filename = filename + extension

	// Ensure 'files' directory exists
	imageDirPath := filepath.Join("files", imagedir)
	err = os.MkdirAll(imageDirPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("Failed to create directory: %v", err)
	}

	filePath := filepath.Join(imageDirPath, filename)
	relativePath := filepath.ToSlash(imagedir + filename) // For replacement in HTML

	// Skip download if file already exists
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("File already exists: %s\n", filePath)
		return relativePath, nil
	}

	// Create HTTP client with custom headers
	client := &http.Client{}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
	req.Header.Set("Accept", "*/*")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Failed to execute HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Failed to download image: %s", resp.Status)
	}

	// Open the output file
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("Failed to create file: %v", err)
	}
	defer out.Close()

	// Ensure the full response body is copied
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to save image to file: %v", err)
	}

	// Check if the content length matches the downloaded file size
	if resp.ContentLength > 0 {
		stat, err := out.Stat()
		if err == nil && stat.Size() != resp.ContentLength {
			return "", fmt.Errorf("File size mismatch: expected %d bytes, got %d bytes", resp.ContentLength, stat.Size())
		}
	}

	fmt.Printf("Downloaded image: %s\n", filePath)
	return relativePath, nil
}

// writeCSV writes the modified records back to a CSV file
func writeCSV(records [][]string, outputFileName string, selectedSeparator string) error {
	file, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Set the separator based on user selection
	switch selectedSeparator {
	case "Comma (,)":
		writer.Comma = ','
	case "Semicolon (;)":
		writer.Comma = ';'
	case "Tab (\\t)":
		writer.Comma = '\t'
	}

	for _, record := range records {
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// showError displays an error dialog
func showError(win fyne.Window, err error) {
	dialog.ShowError(err, win)
}

// showInfo displays an information dialog
func showInfo(win fyne.Window, message string) {
	dialog.ShowInformation("Success", message, win)
}

// updateStatus updates the status binding
func updateStatus(statusBinding binding.String, status string) {
	statusBinding.Set("Status: " + status)
}

// transliterate converts Cyrillic characters to Latin and cleans up the filename
func transliterate(filename string) string {
	var builder strings.Builder

	// Step 1: Transliterate each character
	for _, char := range filename {
		if latin, found := cyrillicToLatin[char]; found {
			builder.WriteString(latin)
		} else {
			builder.WriteRune(char) // Keep original character if not Cyrillic
		}
	}

	// Step 2: Replace forbidden symbols with a dash
	transliterated := builder.String()
	transliterated = strings.ReplaceAll(transliterated, " ", "-")                      // Replace spaces with '-'
	transliterated = regexp.MustCompile(`[^\w-]`).ReplaceAllString(transliterated, "") // Remove all non-word chars except '-'

	// Step 3: Ensure only ASCII letters, numbers, and '-' remain
	transliterated = strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r // Keep ASCII letters, numbers, and '-'
		}
		return -1 // Remove other characters
	}, transliterated)

	// Step 4: Return the cleaned-up filename
	return strings.ToLower(transliterated) // Convert the final filename to lowercase
}
