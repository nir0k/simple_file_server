package main

import (
    "archive/zip"
    "encoding/json"
    "flag"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "os"
    "path"
    "path/filepath"
    "strings"
)

var baseDir string
var templates *template.Template

// Config - structure for storing configuration
type Config struct {
    BaseDir      string `json:"base_dir"`
    Port         string `json:"port"`
    Protocol     string `json:"protocol"` // "http" or "https"
    SSLCertFile  string `json:"ssl_cert_file,omitempty"`
    SSLKeyFile   string `json:"ssl_key_file,omitempty"`
}

func main() {
    // Parsing command line arguments
    configPath := flag.String("config", "config.json", "Path to the configuration file")
    flag.Parse()

    // Reading and parsing the configuration file
    var config Config
    configFile, err := os.Open(*configPath)
    if err != nil {
        log.Fatalf("Error opening configuration file: %v", err)
    }
    defer configFile.Close()

    decoder := json.NewDecoder(configFile)
    err = decoder.Decode(&config)
    if err != nil {
        log.Fatalf("Error parsing configuration file: %v", err)
    }

    // Setting the base directory
    baseDir = config.BaseDir

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
        "getFileInfo": func(fullPath, name string) os.FileInfo {
            info, err := os.Stat(filepath.Join(fullPath, name))
            if err != nil {
                return nil
            }
            return info
        },
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
    templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))

    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))

    // Routes without authentication
    http.HandleFunc("/login", loginHandler)
    http.HandleFunc("/logout", logoutHandler)
    http.HandleFunc("/", fileHandler)
    http.HandleFunc("/download", downloadHandler)
    
    // Routes with authorization for actions
    protected := http.NewServeMux()
    protected.HandleFunc("/upload", uploadHandler)
    protected.HandleFunc("/delete", deleteHandler)
    protected.HandleFunc("/create-folder", createFolderHandler)

    // Apply authorization only to upload, delete, and create actions
    http.Handle("/upload", authMiddlewareForActions(protected))
    http.Handle("/delete", authMiddlewareForActions(protected))
    http.Handle("/create-folder", authMiddlewareForActions(protected))

    addr := ":" + config.Port

    log.Printf("Server started at %s://localhost%s\n", config.Protocol, addr)

    if config.Protocol == "https" {
        if config.SSLCertFile == "" || config.SSLKeyFile == "" {
            log.Fatal("For HTTPS, ssl_cert_file and ssl_key_file must be specified in the configuration")
        }
        log.Fatal(http.ListenAndServeTLS(addr, config.SSLCertFile, config.SSLKeyFile, nil))
    } else {
        log.Fatal(http.ListenAndServe(addr, nil))
    }
}

// renderTemplate - function for rendering a template
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
    err := templates.ExecuteTemplate(w, tmpl, data)
    if err != nil {
        http.Error(w, "Error rendering template", http.StatusInternalServerError)
        log.Println("Error rendering template:", err)
    }
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
    reqPath := r.URL.Path
    fullPath := filepath.Join(baseDir, reqPath)
    info, err := os.Stat(fullPath)
    if err != nil {
        http.NotFound(w, r)
        log.Println("Path not found:", fullPath)
        return
    }

    // Check if this is a directory
    if info.IsDir() {
        // If this is a directory and the slash is missing, redirect with adding a slash
        if !strings.HasSuffix(reqPath, "/") {
            http.Redirect(w, r, reqPath+"/", http.StatusMovedPermanently)
            return
        }

        files, err := os.ReadDir(fullPath)
        if err != nil {
            http.Error(w, "Error reading directory", http.StatusInternalServerError)
            log.Println("Error reading directory:", err)
            return
        }

        var parentDir string
        if reqPath != "/" {
            parentDir = path.Clean("/" + path.Join(reqPath, ".."))
        }

        data := struct {
            Path      string
            FullPath  string
            Files     []os.DirEntry
            ParentDir string
        }{
            Path:      reqPath,
            FullPath:  fullPath,
            Files:     files,
            ParentDir: parentDir,
        }

        renderTemplate(w, "index.html", data)
    } else {
        http.ServeFile(w, r, fullPath)
    }
}

// downloadHandler - handler for file download requests
func downloadHandler(w http.ResponseWriter, r *http.Request) {
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
            log.Println("Error accessing item:", err)
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
                log.Println("Error adding file to ZIP:", err)
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
        return
    }

    files := r.MultipartForm.File["uploadFiles"]
    for _, fileHeader := range files {
        file, err := fileHeader.Open()
        if err != nil {
            http.Error(w, "Error getting file", http.StatusBadRequest)
            return
        }
        defer file.Close()

        dstPath := filepath.Join(fullDestPath, fileHeader.Filename)
        dst, err := os.Create(dstPath)
        if err != nil {
            http.Error(w, "Error saving file", http.StatusInternalServerError)
            return
        }
        defer dst.Close()

        _, err = io.Copy(dst, file)
        if err != nil {
            http.Error(w, "Error saving file", http.StatusInternalServerError)
            return
        }
    }

    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}

// createFolderHandler - handler for creating directories
func createFolderHandler(w http.ResponseWriter, r *http.Request) {
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
        log.Println("Error creating folder:", err)
        return
    }

    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}

// deleteHandler - handler for deleting files and directories
func deleteHandler(w http.ResponseWriter, r *http.Request) {
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
        err := os.RemoveAll(fullPath)
        if err != nil {
            http.Error(w, "Error deleting item", http.StatusInternalServerError)
            log.Println("Error deleting item:", err)
            return
        }
    }

    reqPath := r.FormValue("currentPath")
    http.Redirect(w, r, reqPath, http.StatusSeeOther)
}
