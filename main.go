package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

type HomeData struct {
	BlogData
	Pages []PageData
}

type PageData struct {
	Title    string
	Date     string
	Filename string
}

type BlogData struct {
	Title string
	Date  string
	// scuffed
	Navbar  string
	Content string
}

func (data *BlogData) GetNavbar(w http.ResponseWriter) {
	navbar, err := os.ReadFile("./templates/navbar.html")
	if err != nil {
		http.Error(w, "Navbar file not found", http.StatusNotFound)
		return
	}
	data.Navbar = string(navbar)
}

func main() {
	mux := http.NewServeMux()

	// Define route with wildcard
	mux.HandleFunc("GET /", homeHandler)
	mux.HandleFunc("GET /{name}", pageHandler)
	fileServer := http.FileServer(http.Dir("./static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	port := 9000
	fmt.Println("Server running on http://localhost:", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), logRequest(mux))
	if err != nil {
		log.Fatal(err)
	}
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	data, err := parseMarkdownMetadata("./blogs")
	if err != nil {
		http.Error(w, "What happened", http.StatusNotFound)
		return
	}
	homeData := HomeData{Pages: data}
	homeData.GetNavbar(w)

	// Create and execute template
	t, err := template.ParseFiles("home.html")
	if err != nil {
		log.Fatal(err)
	}

	err = t.Execute(w, homeData)
	if err != nil {
		log.Fatal(err)
	}
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if len(name) == 0 {
		homeHandler(w, r)
		return
	}

	mdPath := filepath.Join("blogs", name+".md")

	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		http.Error(w, "Markdown file not found", http.StatusNotFound)
		return
	}

	// Setup Goldmark with meta extension
	md := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	context := parser.NewContext()

	var buf bytes.Buffer
	if err := md.Convert(mdContent, &buf, parser.WithContext(context)); err != nil {
		http.Error(w, "Error processing markdown", http.StatusInternalServerError)
		return
	}

	// Extract metadata
	metaData := meta.Get(context)
	title := "Untitled"
	date := "01/01/20XX"
	if t, ok := metaData["title"].(string); ok {
		title = t
	}
	if d, ok := metaData["date"].(string); ok {
		date = d
	}

	// Create and execute template
	t, err := template.ParseFiles("./templates/blog.html")
	if err != nil {
		log.Fatal(err)
	}

	data := BlogData{Title: title, Date: date, Content: buf.String()}
	data.GetNavbar(w)

	err = t.Execute(w, data)
	if err != nil {
		log.Fatal(err)
	}
}

func parseMarkdownMetadata(dir string) ([]PageData, error) {
	var metadataList []PageData

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		md := goldmark.New(
			goldmark.WithExtensions(meta.Meta),
		)

		context := parser.NewContext()
		if err := md.Convert(content, io.Discard, parser.WithContext(context)); err != nil {
			return err
		}

		metaData := meta.Get(context)

		// Extract title and date, convert to string if present
		title, _ := metaData["title"].(string)
		date, _ := metaData["date"].(string)

		page := PageData{
			Title:    title,
			Date:     date,
			Filename: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}
		metadataList = append(metadataList, page)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return metadataList, nil
}
