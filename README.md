# File server on Golang
This project is a web application that implements a file manager with user authentication via PAM. The application supports both HTTP and HTTPS protocols.

## **Features**

- **PAM Authentication**: Uses local Linux accounts for user authentication.
- **HTTPS Support**: Secure data transmission using SSL.
- **Configuration**: Application settings are stored in the `config.yaml` file.
- **File Management**: View, upload, download, create, and delete files and folders.

## **Installation**

1. **Clone the repository**

   ```bash
   git clone https://github.com/your_username/your_repository.git
   cd your_repository
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
- `log_severity`: Severity level of logs (e.g., trace, debug, info, warn, error).
- `log_max_size`: Maximum size in megabytes of the log file before it gets rotated.
- `log_max_files`: Maximum number of old log files to retain.
- `log_max_age`: Maximum number of days to retain old log files.

4. **Create an SSL certificate** (if using HTTPS)
For testing purposes, you can create a self-signed certificate:
```bash
openssl req -x509 -newkey rsa:4096 -nodes -out cert.pem -keyout key.pem -days 365
```
Update the paths in `config.yaml` to the corresponding `cert.pem` and `key.pem`.

5. **Build the application**
```bash
go build -o file_manager ./cmd/file_manager
```

6. **Run the application**
```bash
./file_manager -config config.json
```