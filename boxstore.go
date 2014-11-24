package boxstore

import (
	"html/template"
	"io"
	"net/http"

	"appengine"
	"appengine/blobstore"
	"appengine/user"
)

func serveError(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, "Internal Server Error")
	c.Errorf("%v", err)
}

var rootTemplate = template.Must(template.New("root").Parse(rootTemplateHTML))

const rootTemplateHTML = `
<html><body>
<form action="{{.uploadURL}}" method="POST" enctype="multipart/form-data">
Upload File: <input type="file" name="file"><br>
<input type="submit" name="submit" value="Submit">
</form>
<a href="{{.signOutURL}}">sign out</a>
</body>
</html>
`

func handleRoot(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	uploadURL, err := blobstore.UploadURL(c, "/upload", nil)
	if err != nil {
		serveError(c, w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	signOutURL, err := user.LogoutURL(c, "/")
	if err != nil {
		c.Errorf("%v", err)
	}
	err = rootTemplate.Execute(w, map[string]interface{}{
		"uploadURL":  uploadURL,
		"signOutURL": signOutURL,
	})
	if err != nil {
		c.Errorf("%v", err)
	}
}

func handleServe(w http.ResponseWriter, r *http.Request) {
	blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "Forbidden: only admin user can upload files")
		c.Errorf("Non admin user tried to upload files: %v", user.Current(c).Email)
		return
	}

	blobs, _, err := blobstore.ParseUpload(r)
	if err != nil {
		serveError(c, w, err)
		return
	}
	file := blobs["file"]
	if len(file) == 0 {
		c.Errorf("no file uploaded")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/serve/?blobKey="+string(file[0].BlobKey), http.StatusFound)
}

func init() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/serve/", handleServe)
	http.HandleFunc("/upload", handleUpload)
}
