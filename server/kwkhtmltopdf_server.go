package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// TODO ignore opts?
// --log-level, -q, --quiet, --read-args-from-stdin, --dump-default-toc-xsl
// --dump-outline <file>, --allow <path>, --cache-dir <path>,
// --disable-local-file-access, --enable-local-file-access

// TODO sensitive opts to be hidden from log
// --cookie <name> <value>, --password <password>,
// --ssl-key-password <password>

func wkhtmltopdfBin() string {
	bin := os.Getenv("KWKHTMLTOPDF_BIN")
	if bin != "" {
		return bin
	}
	return "wkhtmltopdf"
}

func isDocOption(arg string) bool {
	switch arg {
	case
		"-h",
		"--help",
		"-H",
		"--extended-help",
		"-V",
		"--version",
		"--readme",
		"--license",
		"--htmldoc",
		"--manpage":
		return true
	}
	return false
}

func abortResponse(w http.ResponseWriter) {
	// abort chunked encoding response as crude way to report error to client
	wh, ok := w.(http.Hijacker)
	if !ok {
		log.Println("cannot abort connection, error not reported to client: http.Hijacker not supported")
		return
	}
	c, _, err := wh.Hijack()
	if err != nil {
		log.Println("cannot abort connection, error not reported to client: ", err)
		return
	}
	c.Close()
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/pdf" {
		// handle / and /pdf, keep the rest for future use
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}

	// temp dir for files
	tmpdir, err := ioutil.TempDir("", "kwk")
	if err != nil {
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}
	defer os.RemoveAll(tmpdir)

	// parse request
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var docOutput bool
	var args []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if part.FormName() == "option" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			arg := buf.String()
			args = append(args, arg)
			if isDocOption(arg) {
				docOutput = true
			}
		} else if part.FormName() == "file" {
			// It's important to preserve as much as possible of the
			// original filename because some javascript can depend on it
			// through document.location.
			path := filepath.Join(tmpdir, filepath.Base(part.FileName()))
			// TODO what if multiple files with same basename?
			file, err := os.Create(path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, err = io.Copy(file, part)
			file.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			args = append(args, path)
		} else {
			http.Error(w, "unpexpected part name", http.StatusBadRequest)
			return
		}
	}

	if docOutput {
		w.Header().Add("Content-Type", "text/plain")
	} else {
		w.Header().Add("Content-Type", "application/pdf")
		args = append(args, "-")
	}

	log.Println(args) // TODO better logging, hide sensitve options

	cmd := exec.Command(wkhtmltopdfBin(), args...)
	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, cmdStdout)
	if err != nil {
		abortResponse(w)
		return
	}
	err = cmd.Wait()
	if err != nil {
		abortResponse(w)
		return
	}
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("kwkhtmltopdf server listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
