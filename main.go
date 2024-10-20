package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"

	"path"
	"path/filepath"
	"simple_file_server/pkg"
	"simple_file_server/pkg/auth"
	"simple_file_server/pkg/logger"
	"strings"

	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v2"
)

var baseDir string

// setup - function for setting up the configuration
func setup() (pkg.Config, error) {
    // Parsing command line arguments
    configPath := flag.String("config", "config.yaml", "Path to the configuration file")
    flag.Parse()

    // Reading and parsing the configuration file
    var config pkg.Config
    if _, err := os.Stat(*configPath); os.IsNotExist(err) {
        return config, fmt.Errorf("configuration file not found: %s", *configPath)
    }
    // Reading the configuration file
    configFile, err := os.ReadFile(*configPath)
    if err != nil {
        return config, fmt.Errorf("error opening configuration file: %v", err)
    }
    // Parsing the configuration file
    err = yaml.Unmarshal(configFile, &config)
    if err != nil {
        return config, fmt.Errorf("error parsing configuration file: %v", err)
    }

    // Setting up logging
    logger.LogSetup(config.Logging)

    return config, nil

}


func main() {
    // Setting up configuration
    config, err := setup()
    if err != nil {
        logger.Logger.Fatalf("Error setting up configuration: %v", err)
    }
    // Setting the base directory
    baseDir = config.WebServer.BaseDir
    logger.Logger.Printf("Base directory: %s", baseDir)

    // Defining custom functions for templates
    funcMap := template.FuncMap{
        "splitPath": func(p string) []string {
            return strings.Split(strings.Trim(p, "/"), "/")
        },
        "joinPath": func(base, elem string) string {
            if base == "/" {
                return "/" + elem
            }
            return base + "/" + elem
        },
        "getFileIcon": func(filename string) string {
            ext := strings.ToLower(filepath.Ext(filename))
            switch ext {
            case ".txt":
                return "description"
            case ".pdf":
                return "picture_as_pdf"
            case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
                return "image"
            case ".zip", ".rar", ".7z", ".tar", ".gz":
                return "archive"
            case ".doc", ".docx":
                return "description"
            case ".xls", ".xlsx":
                return "grid_on"
            case ".ppt", ".pptx":
                return "slideshow"
            case ".mp3", ".wav", ".aac":
                return "audiotrack"
            case ".mp4", ".avi", ".mov", ".mkv":
                return "movie"
            default:
                return "insert_drive_file"
            }
        },
        // Function to get file information
        "getFileInfo": func(fullPath, name string) os.FileInfo {
            info, err := os.Stat(filepath.Join(fullPath, name))
            if err != nil {
                logger.Logger.Trace("Error getting file info:", err)
                return nil
            }
            return info
        },
        // Function to get the readable size of the file
        "readableSize": func(info os.FileInfo) string {
            if info == nil {
                return ""
            }
            size := info.Size()
            // Formatting size to a readable format
            const unit = 1024
            if size < unit {
                return fmt.Sprintf("%d B", size)
            }
            div, exp := int64(unit), 0
            for n := size / unit; n >= unit; n /= unit {
                div *= unit
                exp++
            }
            return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
        },
    }

    // Parsing all templates
    pkg.Templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))

    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))

    // Routes without authentication
    http.HandleFunc("/login", auth.LoginHandler)
    http.HandleFunc("/logout", auth.LogoutHandler)
    http.HandleFunc("/check-session", auth.CheckSessionHandler)
    http.HandleFunc("/", fileHandler)
    http.HandleFunc("/download", downloadHandler)
    
    // Routes with authorization for actions
    protected := http.NewServeMux()
    protected.HandleFunc("/upload", uploadHandler)
    protected.HandleFunc("/delete", deleteHandler)
    protected.HandleFunc("/create-folder", createFolderHandler)

    // Apply authorization only to upload, delete, and create actions
    http.Handle("/upload", auth.AuthMiddlewareForActions(protected))
    http.Handle("/delete", auth.AuthMiddlewareForActions(protected))
    http.Handle("/create-folder", auth.AuthMiddlewareForActions(protected))

    addr := ":" + config.WebServer.Port

    logger.Logger.Printf("Server started at %s://localhost%s\n", config.WebServer.Protocol, addr)

    if config.WebServer.Protocol == "https" {
        if config.WebServer.SSLCert == "" || config.WebServer.SSLKey == "" {
            logger.Logger.Fatal("For HTTPS, ssl_cert_file and ssl_key_file must be specified in the configuration")
        }
        logger.Logger.Fatal(http.ListenAndServeTLS(addr, config.WebServer.SSLCert, config.WebServer.SSLKey, nil))
    } else {
        logger.Logger.Fatal(http.ListenAndServe(addr, nil))
    }
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    reqPath := r.URL.Path
    fullPath := filepath.Join(baseDir, reqPath)
    info, err := os.Stat(fullPath)
    if err != nil {
        http.NotFound(w, r)
        logger.Logger.Printf("Path not found: %s from IP: %s", fullPath, clientIP)
        return
    }

    // Determine if the user is logged in
    isLoggedIn := false
    if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
        if auth.IsValidSessionToken(cookie.Value) {
            isLoggedIn = true
        }
    }

    if info.IsDir() {
        if !strings.HasSuffix(reqPath, "/") {
            http.Redirect(w, r, reqPath+"/", http.StatusMovedPermanently)
            return
        }

        files, err := os.ReadDir(fullPath)
        if err != nil {
            http.Error(w, "Error reading directory", http.StatusInternalServerError)
            logger.Logger.Warnf("Error reading directory: %v from IP: %s", err, clientIP)
            return
        }

        var parentDir string
        if reqPath != "/" {
            parentDir = path.Clean("/" + path.Join(reqPath, ".."))
        }

        // Initialize the data struct with an additional field for ReadmeHTML
        data := struct {
            Path       string
            FullPath   string
            Files      []os.DirEntry
            ParentDir  string
            ModTimes   map[string]time.Time
            IsLoggedIn bool
            ReadmeHTML template.HTML // New field
        }{
            Path:       reqPath,
            FullPath:   fullPath,
            Files:      files,
            ParentDir:  parentDir,
            ModTimes:   make(map[string]time.Time),
            IsLoggedIn: isLoggedIn,
            ReadmeHTML: "", // Initialize to empty
        }

        for _, file := range files {
            fileInfo, err := file.Info()
            if err == nil {
                data.ModTimes[file.Name()] = fileInfo.ModTime()
            }
        }

        // Check if readme.md exists in the directory
        readmePath := filepath.Join(fullPath, "README.md")
        if _, err := os.Stat(readmePath); err == nil {
            // readme.md exists, read and convert it
            content, err := os.ReadFile(readmePath)
            if err == nil {
                var buf bytes.Buffer
                if err := goldmark.Convert(content, &buf); err == nil {
                    data.ReadmeHTML = template.HTML(buf.String())
                } else {
                    logger.Logger.Warnf("Error converting Markdown to HTML: %v", err)
                }
            } else {
                logger.Logger.Warnf("Error reading readme.md: %v", err)
            }
        }

        pkg.RenderTemplate(w, "index.html", data)
    } else {
        logger.Logger.Infof("File served: %s to IP: %s", fullPath, clientIP)
        http.ServeFile(w, r, fullPath)
    }
}

// downloadHandler - handler for file download requests
func downloadHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    r.ParseForm()
    items := r.Form["items"]
    if len(items) == 0 {
        http.Error(w, "No files selected for download", http.StatusBadRequest)
        return
    }

    var files []string
    for _, item := range items {
        fullPath := filepath.Join(baseDir, item)
        info, err := os.Stat(fullPath)
        if err != nil {
            logger.Logger.Errorf("error accessing item: %v from IP: %s", err, clientIP)
            continue
        }
        if !info.IsDir() {
            files = append(files, item)
        }
    }

    if len(files) == 0 {
        http.Error(w, "No files selected for download", http.StatusBadRequest)
        return
    }

    if len(files) == 1 {
        fullPath := filepath.Join(baseDir, files[0])
        logger.Logger.Infof("File downloaded: %s by IP: %s", fullPath, clientIP)
        http.ServeFile(w, r, fullPath)
    } else {
        w.Header().Set("Content-Type", "application/zip")
        w.Header().Set("Content-Disposition", "attachment; filename=\"files.zip\"")
        zipWriter := zip.NewWriter(w)
        defer zipWriter.Close()

        for _, file := range files {
            fullPath := filepath.Join(baseDir, file)
            err := addFileToZip(zipWriter, fullPath, file)
            if err != nil {
                logger.Logger.Errorf("error adding file to ZIP: %v", err)
            }
        }
    }
}

// addFileToZip - function for adding a file to a ZIP archive
func addFileToZip(zipWriter *zip.Writer, filepath string, relPath string) error {
    fileToZip, err := os.Open(filepath)
    if err != nil {
        return err
    }
    defer fileToZip.Close()

    info, err := fileToZip.Stat()
    if err != nil {
        return err
    }

    if info.IsDir() {
        // Skip directories
        return nil
    }

    header, err := zip.FileInfoHeader(info)
    if err != nil {
        return err
    }
    header.Name = relPath
    header.Method = zip.Deflate

    writer, err := zipWriter.CreateHeader(header)
    if err != nil {
        return err
    }

    _, err = io.Copy(writer, fileToZip)
    return err
}

// uploadHandler - handler for file upload requests
func uploadHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    user := r.Header.Get("X-User")
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    err := r.ParseMultipartForm(100 << 20) // 100 MB
    if err != nil {
        http.Error(w, "Error parsing form", http.StatusBadRequest)
        return
    }

    reqPath := r.FormValue("currentPath")
    fullDestPath := filepath.Join(baseDir, reqPath)

    err = os.MkdirAll(fullDestPath, os.ModePerm)
    if err != nil {
        http.Error(w, "Error creating directory", http.StatusInternalServerError)
        logger.Logger.Errorf("Error creating directory: %v from IP: %s, User: %s", err, clientIP, user)
        return
    }

    files := r.MultipartForm.File["uploadFiles"]
    for _, fileHeader := range files {
        file, err := fileHeader.Open()
        if err != nil {
            http.Error(w, "Error getting file", http.StatusBadRequest)
            logger.Logger.Errorf("Error getting file: %v from IP: %s, User: %s", err, clientIP, user)
            return
        }
        defer file.Close()

        dstPath := filepath.Join(fullDestPath, fileHeader.Filename)
        dst, err := os.Create(dstPath)
        if err != nil {
            http.Error(w, "Error saving file", http.StatusInternalServerError)
            logger.Logger.Errorf("Error saving file: %v from IP: %s, User: %s", err, clientIP, user)
            return
        }
        defer dst.Close()

        _, err = io.Copy(dst, file)
        if err != nil {
            http.Error(w, "Error saving file", http.StatusInternalServerError)
            logger.Logger.Errorf("Error saving file: %v from IP: %s, User: %s", err, clientIP, user)
            return
        }
        logger.Logger.Infof("File uploaded: %s by IP: %s, User: %s", dstPath, clientIP, user)
    }

    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}

// createFolderHandler - handler for creating directories
func createFolderHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    user := r.Header.Get("X-User")
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    reqPath := r.FormValue("currentPath")
    folderName := r.FormValue("folderName")
    if folderName == "" {
        http.Error(w, "Folder name is required", http.StatusBadRequest)
        return
    }

    fullPath := filepath.Join(baseDir, reqPath, folderName)

    err := os.Mkdir(fullPath, os.ModePerm)
    if err != nil {
        http.Error(w, "Error creating folder", http.StatusInternalServerError)
        logger.Logger.Errorf("Error creating folder: %v from IP: %s, User: %s", err, clientIP, user)
        return
    }
    logger.Logger.Infof("Folder created: %s by IP: %s, User: %s", fullPath, clientIP, user)

    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}

// deleteHandler - handler for deleting files and directories
func deleteHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    user := r.Header.Get("X-User")
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    r.ParseForm()
    items := r.Form["items"]
    if len(items) == 0 {
        http.Error(w, "No items selected for deletion", http.StatusBadRequest)
        return
    }

    for _, item := range items {
        fullPath := filepath.Join(baseDir, item)
        err := logAndRemoveAll(fullPath, clientIP, user)
        if err != nil {
            http.Error(w, "Error deleting item", http.StatusInternalServerError)
            logger.Logger.Errorf("Error deleting item: %v from IP: %s, User: %s", err, clientIP, user)
            return
        }
        logger.Logger.Infof("Item deleted: %s by IP: %s, User: %s", fullPath, clientIP, user)
    }

    reqPath := r.FormValue("currentPath")
    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}

// logAndRemoveAll - recursive function to log and remove all files and directories
func logAndRemoveAll(path, clientIP, user string) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }

    if info.IsDir() {
        entries, err := os.ReadDir(path)
        if err != nil {
            return err
        }

        for _, entry := range entries {
            err = logAndRemoveAll(filepath.Join(path, entry.Name()), clientIP, user)
            if err != nil {
                return err
            }
        }
    }

    logger.Logger.Infof("Deleting: %s by IP: %s, User: %s", path, clientIP, user)
    return os.RemoveAll(path)
}