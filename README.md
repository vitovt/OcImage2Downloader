# Google Spreadsheet Image Downloader

This application allows users to download images from a product description `<img src=...` tag. Input source is Google Spreadsheet table, where image URLs are embedded within the `description_uk` and `description_ru` columns of spreadsheet. The application processes the CSV data from the Google Spreadsheet, downloads the images, and updates html with new, relative `img src=` path, writing result to the CSV file. It provides a graphical user interface (GUI) built using the [Fyne](https://fyne.io/) toolkit and includes a progress bar to track the download progress.

## Features

- **GUI Interface**: A simple and easy-to-use interface to enter the Google Spreadsheet URL, image directory, and output CSV file.
- **CSV Support**: Processes CSV data, identifying image URLs within the spreadsheet, and downloading the corresponding images.
- **Automatic URL Optimization**: The application transliterates Cyrillic characters to Latin equivalents for better URL optimization and removes forbidden characters.
- **File Handling**: Handles images with the same filename by generating unique filenames using parts of the URL and a hash to avoid overwriting.
- **Progress Bar**: Tracks and displays the download progress of images.
- **Retry Logic**: Retries downloading images in case of temporary network errors (like stream errors or timeouts).
- **Concurrency**: Downloads images concurrently, making the process faster.

## How It Works

1. **Input Spreadsheet**: You input a Google Spreadsheet URL (shared as CSV), a hostname (for constructing full URLs), and a directory to store the images.
2. **Process CSV**: The application downloads the CSV data, extracts image URLs, and begins downloading the images.
3. **Image Download**: Each image URL is downloaded to the specified directory. The application handles file naming conflicts by generating unique filenames for images with the same name but different URLs.
4. **Filename Sanitization**: The filenames are transliterated from Cyrillic (Ukrainian and Russian) characters to Latin letters, and forbidden symbols are removed to ensure the filenames are safe for use in URLs.
5. **CSV Update**: After downloading, the CSV can be updated with the new image paths, ensuring that the image locations are correctly reflected in the data.
6. **Completion Status**: Once all images are downloaded, the progress is displayed, including the number of images downloaded successfully.

## Installation

To use this application, you will need to have Go installed on your system. Follow the instructions below to set up and run the application.

### Build the application

Clone this repository to your local machine:

```bash
git clone https://github.com/vitovt/OcImage2Downloader.git
cd OcImage2Downloader
```

Use gnu make to build this application
```
make build
```

### Run the Application

To run the application, execute ELF or EXE binaries in bin/ directory after compilation

```bash
bin/OCImage2Downloader-v0.99.9_linux_amd64
```

The application will launch the GUI where you can input the necessary information to start downloading images.

Or download binaries from github RELEASE section

## Usage

1. **Spreadsheet URL**: Paste the URL of the Google Spreadsheet you want to process (in CSV format).
2. **Hostname**: Enter the hostname (e.g., `https://site.com.ua`) that will be used to construct full image URLs for relative paths.
3. **Image Directory**: Specify the directory where the images will be downloaded (e.g., `/content/uploads/images/`).
4. **Output CSV File**: Specify the output CSV filename where the updated paths for images will be stored.
5. **CSV Separator**: Choose the separator used in your CSV file (comma, semicolon, or tab).
6. **Process Images**: Click the "Process Images" button to start downloading images. The progress bar will display the current progress.

Once the process is complete, a success message will be shown, and the updated CSV file will be saved with the new image paths.

### Example

- **Spreadsheet URL**: `https://docs.google.com/spreadsheets/d/abcd1234/export?format=csv`
- **Hostname**: `https://example.com`
- **Image Directory**: `/content/uploads/images/`
- **Output CSV File**: `output.csv`

After processing, the images will be downloaded into `/content/uploads/images/`, and the CSV file will contain the updated image paths.

## Error Handling

Detailed log and error descriptions you can see in CLI when you run it from terminal, not in GUI

## Contributing

If you'd like to contribute to the project, feel free to fork the repository and submit a pull request with your improvements.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

