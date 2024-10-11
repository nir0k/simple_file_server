package main

import (
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

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
const sessionCookieName = "session_token"
const sessionDuration = time.Hour * 1 // Session duration 1 hour

// pamAuthenticate - performs user authentication using PAM
func pamAuthenticate(username, password string) error {
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

// generateSessionToken - generates a random token for the session
func generateSessionToken() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// isValidSessionToken - checks the validity of the session token
func isValidSessionToken(token string) bool {
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

// authMiddlewareForActions - protects routes for certain actions
func authMiddlewareForActions(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie(sessionCookieName)
        if err != nil || !isValidSessionToken(cookie.Value) {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

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

// loginHandler - handles /login routes
func loginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
        // Display the login form
        renderTemplate(w, "login.html", nil)
    } else if r.Method == "POST" {
        // Process form data
        username := r.FormValue("username")
        password := r.FormValue("password")

        // Authenticate the user using PAM
        err := pamAuthenticate(username, password)
        if err != nil {
            data := struct {
                Error string
            }{
                Error: "Authentication failed. Please try again.",
            }
            renderTemplate(w, "login.html", data)
            return
        }

        // Authentication was successful
        sessionToken := generateSessionToken()
        expiresAt := time.Now().Add(sessionDuration)
        sessions[sessionToken] = UserSession{
            Username: username,
            Expires:  expiresAt,
        }

        // Set the session cookie
        http.SetCookie(w, &http.Cookie{
            Name:     sessionCookieName,
            Value:    sessionToken,
            Path:     "/",
            Expires:  expiresAt,
            HttpOnly: true,
        })

        http.Redirect(w, r, "/", http.StatusSeeOther)
    } else {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// logoutHandler - handles /logout route
func logoutHandler(w http.ResponseWriter, r *http.Request) {
    // Delete the session
    cookie, err := r.Cookie(sessionCookieName)
    if err == nil {
        delete(sessions, cookie.Value)
        // Delete the cookie
        http.SetCookie(w, &http.Cookie{
            Name:     sessionCookieName,
            Value:    "",
            Path:     "/",
            Expires:  time.Now().Add(-1 * time.Hour),
            HttpOnly: true,
        })
    }
    http.Redirect(w, r, "/login", http.StatusSeeOther)
}
