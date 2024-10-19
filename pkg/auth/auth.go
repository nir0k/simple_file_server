package auth

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"simple_file_server/pkg"
	"simple_file_server/pkg/logger"

	"github.com/msteinert/pam"    
)

// UserSession - represents a user session
type UserSession struct {
    Username string
    Expires  time.Time
}

// sessions - stores active user sessions
var sessions = make(map[string]UserSession)

// Configuration for sessions
const SessionCookieName = "session_token"
const sessionDuration = time.Hour * 24 // Session duration 1 hour

// PamAuthenticate - performs user authentication using PAM
func PamAuthenticate(username, password string) error {
    tx, err := pam.StartFunc("", username, func(s pam.Style, msg string) (string, error) {
        switch s {
        case pam.PromptEchoOff:
            return password, nil
        case pam.PromptEchoOn:
            return password, nil
        case pam.ErrorMsg:
            log.Println("PAM Error:", msg)
            return "", nil
        case pam.TextInfo:
            log.Println("PAM Info:", msg)
            return "", nil
        default:
            return "", fmt.Errorf("unknown PAM message style")
        }
    })
    if err != nil {
        return err
    }
    return tx.Authenticate(0)
}

// GenerateSessionToken - generates a random token for the session
func GenerateSessionToken() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// IsValidSessionToken - checks the validity of the session token
func IsValidSessionToken(token string) bool {
    session, exists := sessions[token]
    if (!exists) {
        return false
    }
    if session.Expires.Before(time.Now()) {
        delete(sessions, token)
        return false
    }
    return true
}

// AuthMiddlewareForActions - protects routes for certain actions
func AuthMiddlewareForActions(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie(SessionCookieName)
        if err != nil || !IsValidSessionToken(cookie.Value) {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

        // Извлекаем имя пользователя из сессии
        session := sessions[cookie.Value]
        r.Header.Set("X-User", session.Username)

        // Check if the user is trying to perform an action that requires authorization
        if r.Method == "POST" && (strings.HasPrefix(r.URL.Path, "/upload") ||
            strings.HasPrefix(r.URL.Path, "/delete") ||
            strings.HasPrefix(r.URL.Path, "/create-folder")) {
            // If the request is POST and directed to upload, delete, or create folder, check authorization
            next.ServeHTTP(w, r)
        } else {
            // If it is a GET request or another action that does not require authorization, allow access
            next.ServeHTTP(w, r)
        }
    })
}

// LoginHandler - handles /login routes
func LoginHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    if r.Method == "GET" {
        // Display the login form
        pkg.RenderTemplate(w, "login.html", nil)
    } else if r.Method == "POST" {
        // Process form data
        username := r.FormValue("username")
        password := r.FormValue("password")

        // Authenticate the user using PAM
        err := PamAuthenticate(username, password)
        if err != nil {
            data := struct {
                Error string
            }{
                Error: "Authentication failed. Please try again.",
            }
            pkg.RenderTemplate(w, "login.html", data)
            logger.Logger.Warnf("Authentication failed for user: %s from IP: %s", username, clientIP)
            return
        }

        // Authentication was successful
        sessionToken := GenerateSessionToken()
        expiresAt := time.Now().Add(sessionDuration)
        sessions[sessionToken] = UserSession{
            Username: username,
            Expires:  expiresAt,
        }

        // Set the session cookie
        http.SetCookie(w, &http.Cookie{
            Name:     SessionCookieName,
            Value:    sessionToken,
            Path:     "/",
            Expires:  expiresAt,
            HttpOnly: true,
        })

        logger.Logger.Infof("User %s logged in successfully from IP: %s", username, clientIP)
        http.Redirect(w, r, "/", http.StatusSeeOther)
    } else {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// LogoutHandler - handles /logout routes
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    // Delete the session
    cookie, err := r.Cookie(SessionCookieName)
    if err == nil {
        delete(sessions, cookie.Value)
        // Delete the cookie
        http.SetCookie(w, &http.Cookie{
            Name:     SessionCookieName,
            Value:    "",
            Path:     "/",
            Expires:  time.Now().Add(-1 * time.Hour),
            HttpOnly: true,
        })
        logger.Logger.Infof("User logged out successfully from IP: %s", clientIP)
    }
    // Возвращаем пользователя на предыдущую страницу
    referer := r.Referer()
    if referer == "" || referer == "/logout" {
        referer = "/"
    }
    http.Redirect(w, r, referer, http.StatusSeeOther)
}

// CheckSessionHandler - проверяет, действительна ли сессия
func CheckSessionHandler(w http.ResponseWriter, r *http.Request) {
    cookie, err := r.Cookie(SessionCookieName)
    if err != nil || !IsValidSessionToken(cookie.Value) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    w.WriteHeader(http.StatusOK)
}
