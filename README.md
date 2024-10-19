# Simple File Manager
This project is a web application that implements a file manager with user authentication via PAM. The application supports HTTP and HTTPS protocols.

## **Features**

- **PAM Authentication**: Uses local Linux accounts for user authentication.
- **HTTPS Support**: Secure data transmission using SSL.
- **Configuration**: Application settings are stored in the `config.yaml` file.
- **File Management**: View, upload, download, create, and delete files and folders.

## Installation
- **Go**: Version 1.22 or higher.
- **libpam0g-dev**: Libraries for PAM support.
### Prerequisites

### Installation Steps
1. **Clone the repository**

   ```bash
   git clone https://github.com/your_username/simple-file-manager.git
   cd simple-file-manager
   ```

2. **Install dependencies**

   ```bash
   go mod tidy
   ```

3. **Create a configuration file**
Create a `config.yaml` file in the project root with the following content:
  
   ```yaml
   web-server:
      base_dir: "/path/to/your/directory"
      port: "8080"
      protocol: "https"
      ssl_cert_file: "/path/to/certificate/cert.pem"
      ssl_key_file: "/path/to/key/key.pem"
   logging:
      log_file: "log/log.json"
      log_severity: "trace"
      log_max_size: 10
      log_max_files: 10
      log_max_age: 10
   ```
- `base_dir`: Base directory for the file manager.
- `port`: Port on which the server will run.
- `protocol`: Protocol (http or https).
- `ssl_cert_file` and `ssl_key_file`: Paths to the SSL certificate and key (required when using HTTPS).
- `log_file`: Path to the log file.
- `log_severity`: Log severity level (e.g., trace, debug, info, warn, error).
- `log_max_size`: Maximum log file size in megabytes before rotation.
- `log_max_files`: Maximum number of old log files to retain.
- `log_max_age`: Maximum number of days to retain old log files.

4. **Create an SSL certificate** (if using HTTPS)

   For testing, you can create a self-signed certificate:
   ```bash
   openssl req -x509 -newkey rsa:4096 -nodes -out cert.pem -keyout key.pem -days 365
   ```
   Ensure the paths to `cert.pem` and `key.pem` in `config.yaml` are correct.

5. **Build the application**
   ```bash
   go build -o file_server .
   ```

6. **Run the application**
   ```bash
   ./file_server -config config.yaml
   ```
   Or, if using the default path for
   ```bash
   ./file_server
   ```

## Docker
You can also build the application using Docker for Debian 10.
**Note**: Docker is used only for building the package.

1. Build the Docker image and extract the binary file
   ```bash
   ./build.sh
   ```
2. Run the application
   ```bash
   ./file_server -config config.yaml
   ```

## Usage

- Open a browser and go to http(s)://localhost:8080 (or use the port specified in the configuration).
- Use the web interface to manage files and folders:

   - **Upload Files**: Click "Upload Files" and select files to upload.
   - **Create Folder**: Click "Create Folder" and enter the name of the new folder.
   - **Delete**: Select files or folders and click "Delete".
   - **Download**: Select files and click "Download Selected Files".

## Notes
- **PAM Authentication**: Ensure PAM is properly configured on your system.
- **Access Rights**: The application needs read and write permissions in the specified `base_dir`.
- **Logging**: Logs are saved to the file specified in `log_file`. Configure parameters in the `logging` section of the `config.yaml` file.

## Themes

- Switching between light and dark themes is available through the icon in the top right corner of the interface.
- The selected theme is saved in the browser's `localStorage`.

## Displaying README.md
- If a `README.md` file is present in the current directory, it will be automatically displayed as HTML at the bottom of the page.