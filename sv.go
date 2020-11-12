package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

const (
	bytesInMB        = 1000 * 1000
	defaultDirectory = "."
	fpsep            = string(filepath.Separator)
)

var (
	directory, host     string
	port, maxInlineSize int
	forbidden           string
	lazy, help, quiet   bool
)

func init() {
	pflag.StringVarP(&host, "host", "a", "0.0.0.0", "Host address. Default broadcasts to public. Use '127.0.0.1' or 'localhost' for local broadcasting.")
	pflag.StringVarP(&directory, "dir", "d", defaultDirectory, "Folder to be broadcast")
	pflag.IntVarP(&port, "port", "p", 8080, "Address on which server is broadcasted")
	pflag.IntVarP(&maxInlineSize, "inlinesize", "k", 24, "Max size of file in MB before being downloaded as attachment (see 'Content-Disposition')")
	pflag.StringVarP(&forbidden, "exclude", "x", `^\.`, "Exclude directories with matching regexp pattern")
	pflag.BoolVarP(&lazy, "lazy", "l", true, "Enables lazy loading of files. caution: if false will load all files to memory on startup")
	pflag.BoolVarP(&help, "help", "h", false, "Call help")
	pflag.BoolVarP(&quiet, "quiet", "q", false, "Run sv quietly (no output).")
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
	if os.Chdir(directory) == nil {
		directory = "."
	}
	re, err := regexp.Compile(forbidden)
	if err != nil {
		return err
	}
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		path = strings.ReplaceAll(path, fpsep, "/")
		if info.IsDir() {
			return nil
		}
		dir, file := filepath.Split(path)
		folders := strings.Split(dir, "/")
		for _, folder := range folders {
			if re.MatchString(folder) {
				return nil
			}
		}
		ep := endpoint{path: path, contentType: getContentType(file)}
		if !lazy {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			ep.content, err = ioutil.ReadAll(f)
			if err != nil {
				return err
			}
		}
		http.Handle(ep.address(), &ep)
		printf("add %s on http://%s:%d%s", file, host, port, ep.address())
		return nil
	})
	if err != nil {
		return err
	}
	infof("done. listening and serving...")
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
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
		fmt.Printf("\t-%s,  --%s   %s (default %s)\n ", flag.Shorthand, flag.Name, flag.Usage, flag.DefValue)
	})
}

type endpoint struct {
	path        string
	contentType string
	content     []byte
}

func (ep endpoint) address() (adress string) {
	if ep.fileName() == "index.html" {
		return fmt.Sprintf("/%s", strings.TrimSuffix(ep.path, ep.fileName()))
	}
	return fmt.Sprintf("/%s", ep.path)
}

func (ep endpoint) fileName() string {
	_, file := filepath.Split(ep.path)
	return file
}

func (ep *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var f *os.File
	var err error
	if !lazy {
		w.Header().Add("Content-Type", ep.contentType)
		w.Write(ep.content)
		return
	}
	f, err = os.Open(ep.path)
	if err != nil {
		errorf("%s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	info, err := f.Stat()
	if err != nil {
		errorf("%s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if info.Size() > int64(maxInlineSize*bytesInMB) {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, ep.fileName()))
	}
	w.Header().Add("Content-Type", ep.contentType)
	w.Header().Set("Content-Length", strconv.Itoa(int(info.Size())))
	n, err := io.Copy(w, f)
	if err != nil {
		errorf("while serving %s, %d bytes copied: %s", ep.path, n, err)
	}
	return
}

func infof(format string, args ...interface{})  { logf("inf", format, args) }
func printf(format string, args ...interface{}) { logf("srv", format, args) }
func errorf(format string, args ...interface{}) { logf("err", format, args) }
func logf(tag, format string, args []interface{}) {
	if !quiet {
		msg := fmt.Sprintf(format, args...)
		if args == nil {
			msg = fmt.Sprintf(format)
		}
		fmt.Println(fmt.Sprintf("[%s] %s", tag, msg))
	}
}

func getContentType(filename string) string {
	var contentType string
	ext := filepath.Ext(filename)
	ext = strings.Replace(ext, ".", "", 1)
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
	case "go", "h", "c", "py", "tex", "sty", "m", "sum", "mod", "lock": // program
		contentType = "text/plain"
	default:
		contentType = "application/octet-stream"
	}
	if strings.Contains(contentType, "text/") {
		contentType += "; charset=utf-8\n\n"
	}
	return contentType
}
