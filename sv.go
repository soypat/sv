package main

import (
	"errors"
	"fmt"
	"github.com/spf13/pflag"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const defaultDirectory = "."

var (
	directory string
	port      int
	forbidden string
	lazy, help      bool
)
func init() {
	pflag.StringVarP(&directory, "dir", "d", defaultDirectory, "Folder to be broadcast")
	pflag.IntVarP(&port, "port", "p", 8080, "Address on which server is broadcasted")
	pflag.StringVarP(&forbidden, "exclude", "x", "^\\.", "Exclude directories with matching regexp pattern")
	pflag.BoolVarP(&lazy, "lazy", "l", false, "Enables lazy loading of files")
	pflag.BoolVarP(&help, "help","h",false, "Call help")
	pflag.Lookup("help").Hidden = true
	pflag.Parse()
	if help {
		printHelp()
		os.Exit(0)
	}
}
func run() error {
	f, err := os.Stat(directory)
	if err != nil {
		return err
	} else if !f.IsDir() {
		return errors.New(f.Name() + " is not a directory")
	}
	re, err := regexp.Compile(forbidden)
	if err != nil {
		return err
	}
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		dir, file := filepath.Split(path)
		folders := strings.Split(dir, string(filepath.Separator))
		for _, folder := range folders {
			if re.MatchString(folder) {
				return nil
			}
		}
		if file == "index.html" {
			fmt.Printf("[srv] %s accesible on localhost:%d/%s\n",file,port,dir)
			http.Handle("/"+dir, &endpoint{path: path, contentType: getContentType(file)})
		} else {
			fmt.Printf("[srv] %s accesible on localhost:%d/%s\n",file,port,path)
			http.Handle("/"+path, &endpoint{path: path, contentType: getContentType(file)})
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("[inf] done. listening and serving...")
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}


func main() {
	if err := run(); err != nil {
		fmt.Println("[err] ", err)
		os.Exit(1)
	}
}
const helpMsg = `sv is a tool to run a http server easy-peasy.

Usages:
	sv [flags]
Flags:`
func printHelp() {
	fmt.Println(helpMsg)
	pflag.VisitAll(func(flag *pflag.Flag) {
		fmt.Printf("\t-%s,  --%s   %s (default %s)\n ",flag.Shorthand,flag.Name,flag.Usage,flag.DefValue )
	})
}

type endpoint struct {
	path        string
	contentType string
	content     []byte
	once        sync.Once
}

func (ep *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var f *os.File
	var err error
	if !lazy {
		ep.once.Do(func() {
			f, _ = os.Open(ep.path)
			ep.content, err = ioutil.ReadAll(f)
		})
		if err != nil {
			panic(err)
		}
		w.Header().Add("Content-Type", ep.contentType)
		w.Write(ep.content)
		return
	}
	f, err = os.Open(ep.path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", ep.contentType)
	w.Write(b)
}

func getContentType(filename string) string {
	var contentType string
	fileTypeIndex := strings.LastIndex(filename, ".")
	if fileTypeIndex == -1 || len(filename) == fileTypeIndex+1 { // si el nombre termina con un punto (is that even legal?)
		contentType = "application/octet-stream"
		return contentType // Or next part errors!
	}
	ext := filename[fileTypeIndex+1:] // file extension
	switch ext {
	// Typical Web stuff
	case "js", "mjs":
		contentType = "application/javascript"
	case "css", "csv":
		contentType = "text/" + ext
	case "html", "htm":
		contentType = "text/html"

		// APPLICATION AND STUFF
	case "7z":
		contentType = "application/x-7z-compressed"
	case "zip", "rtf", "json", "xml", "pdf":
		contentType = "application/" + ext
	case "gz":
		contentType = "application/gzip"
	case "rar":
		contentType = "application/vnd.rar"
	case "doc":
		contentType = "application/msword"
	case "docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "ppt":
		contentType = "application/vnd.ms-powerpoint"
	case "pptx":
		contentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xls":
		contentType = "application/vnd.ms-excel"
	case "xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xhtml":
		contentType = "application/xhtml+xml"
	case "sh", "csh":
		contentType = "application/x-" + ext

		// FONT
	case "ttf", "otf", "woff", "woff2":
		contentType = "font/" + ext

		// AUDIO
	case "wav", "aac", "opus":
		contentType = "audio/" + ext
	case "mp3":
		contentType = "audio/mpeg"

		// IMAGE
	case "bmp", "gif", "png", "webp":
		contentType = "image/" + ext
	case "tif", "tiff":
		contentType = "image/tiff"
	case "svg":
		contentType = "image/svg+xml"
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "ico":
		contentType = "image/x-icon"

		// VIDEO
	case "ts":
		contentType = "video/mp2t"
	case "avi":
		contentType = "video/x-msvideo"
	case "mp4", "webm", "mpeg":
		contentType = "video/" + ext

		// Plaintext
	case "txt", "dat", "md", ".gitignore":
		contentType = "text/plain"
	case "go", "h", "c", "py", "tex", "sty", "m": // program
		contentType = "text/plain"
	default:
		contentType = "application/octet-stream"
	}
	return contentType
}
