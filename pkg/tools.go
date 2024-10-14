// Description: This file contains the RenderTemplate function which is used to render the templates.
package pkg

import (
    "html/template"
    "net/http"
    "log"
)

var Templates *template.Template

// RenderTemplate - renders the template with the provided data
func RenderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
    err := Templates.ExecuteTemplate(w, tmpl, data)
    if err != nil {
        http.Error(w, "Error rendering template", http.StatusInternalServerError)
        log.Println("Error rendering template:", err)
    }
}
