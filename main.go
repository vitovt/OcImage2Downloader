package main

import (
    "bufio"
    "encoding/csv"
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

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/data/binding"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myApp := app.New()
    myWindow := myApp.NewWindow("Google Spreadsheet Image Downloader")

    // Input fields
    urlEntry := widget.NewEntry()
    urlEntry.SetPlaceHolder("Enter Google Spreadsheet URL")

    hostnameEntry := widget.NewEntry()
    hostnameEntry.SetPlaceHolder("Hostname (e.g., https://site.com.ua)")
    hostnameEntry.SetText("https://site.com.ua") // Default value

    imagedirEntry := widget.NewEntry()
    imagedirEntry.SetPlaceHolder("Image Directory (e.g., /content/uploads/images/)")
    imagedirEntry.SetText("/content/uploads/images/") // Default value

    // Data bindings
    progressBinding := binding.NewFloat()
    statusBinding := binding.NewString()
    bodyUkData := binding.NewString()
    bodyRuData := binding.NewString()

    // Progress Bar and Status Label
    progressBar := widget.NewProgressBarWithData(progressBinding)
    statusLabel := widget.NewLabelWithData(statusBinding)

    // Text Areas for Modified Data
    bodyUkText := widget.NewMultiLineEntry()
    bodyUkText.Bind(bodyUkData)
    bodyUkText.SetPlaceHolder("Modified body_uk content will appear here")
    bodyUkText.Disable()

    bodyRuText := widget.NewMultiLineEntry()
    bodyRuText.Bind(bodyRuData)
    bodyRuText.SetPlaceHolder("Modified body_ru content will appear here")
    bodyRuText.Disable()

    // Initialize bindings
    progressBinding.Set(0)
    statusBinding.Set("Status: Idle")

    // Process Button
    processButton := widget.NewButton("Process Images", func() {
        spreadsheetURL := urlEntry.Text
        hostname := hostnameEntry.Text
        imagedir := imagedirEntry.Text

        if spreadsheetURL == "" || hostname == "" || imagedir == "" {
            showError(myWindow, errors.New("Please fill in all required fields"))
            return
        }

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
            err = processRecords(records, hostname, imagedir, progressBinding, statusBinding, bodyUkData, bodyRuData)
            if err != nil {
                showError(myWindow, err)
                updateStatus(statusBinding, "Status: Idle")
                return
            }

            updateStatus(statusBinding, "Status: Completed")
            showInfo(myWindow, "Images downloaded and data processed successfully")
        }()
    })

    // Organize UI Elements
    content := container.NewVBox(
        urlEntry,
        hostnameEntry,
        imagedirEntry,
        processButton,
        progressBar,
        statusLabel,
        widget.NewLabel("Modified body_uk Data:"),
        container.NewVSplit(bodyUkText, bodyRuText),
    )

    myWindow.SetContent(content)
    myWindow.Resize(fyne.NewSize(800, 600))
    myWindow.ShowAndRun()
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
func processRecords(records [][]string, hostname, imagedir string, progressBinding binding.Float, statusBinding binding.String, bodyUkData, bodyRuData binding.String) error {
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

    totalRows := len(records) - 1
    progressBinding.Set(0)

    var allImageLinks []string

    bodyUkResults := make([]string, totalRows)
    bodyRuResults := make([]string, totalRows)

    // Collect all image links
    imageLinkSet := make(map[string]struct{})
    for rowIndex, row := range records[1:] {
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

    updateStatus(statusBinding, "Downloading images...")
    // Download images
    imagePathMap := make(map[string]string)
    var mu sync.Mutex

    var wg sync.WaitGroup
    for _, link := range allImageLinks {
        wg.Add(1)
        go func(link string) {
            defer wg.Done()
            newPath, err := downloadAndSaveImage(link, hostname, imagedir)
            if err != nil {
                fmt.Printf("Error downloading image %s: %v\n", link, err)
                return
            }
            mu.Lock()
            imagePathMap[link] = newPath
            mu.Unlock()
        }(link)
    }
    wg.Wait()

    // Replace image URLs in body_uk and body_ru
    for rowIndex, row := range records[1:] {
        bodyUk := row[headerMap["body_uk"]]
        bodyRu := row[headerMap["body_ru"]]

        newBodyUk := replaceImageLinks(bodyUk, imagePathMap)
        newBodyRu := replaceImageLinks(bodyRu, imagePathMap)

        bodyUkResults[rowIndex] = newBodyUk
        bodyRuResults[rowIndex] = newBodyRu

        // Update progress bar
        progressBinding.Set(float64(rowIndex+1) / float64(totalRows))
    }

    // Update the text boxes with new data
    bodyUkData.Set(strings.Join(bodyUkResults, "\n"))
    bodyRuData.Set(strings.Join(bodyRuResults, "\n"))

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
func downloadAndSaveImage(imageURL, hostname, imagedir string) (string, error) {
    // Prepare the image URL
    if strings.HasPrefix(imageURL, "//") {
        imageURL = "https:" + imageURL
    } else if !strings.HasPrefix(imageURL, "http") {
        imageURL = strings.TrimRight(hostname, "/") + "/" + strings.TrimLeft(imageURL, "/")
    }

    // Prepare the filename
    parsedURL, err := url.Parse(imageURL)
    if err != nil {
        return "", err
    }
    filename := filepath.Base(parsedURL.Path)
    filename = strings.Split(filename, "?")[0] // Remove query params

    // Ensure 'files' directory exists
    err = os.MkdirAll("files", os.ModePerm)
    if err != nil {
        return "", err
    }

    filePath := filepath.Join("files", filename)
    relativePath := filepath.ToSlash(imagedir + filename) // For replacement in HTML

    // Skip download if file already exists
    if _, err := os.Stat(filePath); err == nil {
        return relativePath, nil
    }

    // Create HTTP client with custom headers
    client := &http.Client{}
    req, err := http.NewRequest("GET", imageURL, nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
    req.Header.Set("Accept", "*/*")

    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("Failed to download image: %s", resp.Status)
    }

    out, err := os.Create(filePath)
    if err != nil {
        return "", err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return "", err
    }

    fmt.Printf("Downloaded image: %s\n", filePath)
    return relativePath, nil
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

