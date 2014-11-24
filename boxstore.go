package boxstore

import (
	"html/template"
	"io"
	"net/http"

	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/user"
)

type BlobFile struct {
	BlobKey  appengine.BlobKey
	Filename string
}

func serveError(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, "Internal Server Error")
	c.Errorf("%v", err)
}

var rootTemplate = template.Must(template.New("root").Parse(rootTemplateHTML))

const rootTemplateHTML = `
<html>
<head>
<meta charset="UTF-8">
</head>
<body>
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
	c := appengine.NewContext(r)

	filename := r.URL.Path[len("/serve/"):]
	c.Infof("filename=%v", filename)

	if filename == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "filename must be specified")
		c.Errorf("filename must be specified")
		return
	}

	key := blobFileKeyFromFilename(c, filename)
	var blobFile BlobFile
	err := datastore.Get(c, key, &blobFile)
	if err == datastore.ErrNoSuchEntity {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "filename not found")
		c.Errorf("filename not found: %s", filename)
		return
	} else if err != nil {
		serveError(c, w, err)
		return
	}

	blobstore.Send(w, blobFile.BlobKey)
}

var uploadDoneTemplate = template.Must(template.New("uploadDone").Parse(uploadDoneTemplateHTML))

const uploadDoneTemplateHTML = `
<html>
<head>
<meta charset="UTF-8">
</head>
<body>
<h1>Upload done!</h1>
<a href="{{.url}}">{{.filename}}</a>
</body>
</html>
`

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

	err = saveBlobFile(c, file[0])
	if err != nil {
		serveError(c, w, err)
		return
	}

	filename := file[0].Filename
	err = uploadDoneTemplate.Execute(w, map[string]interface{}{
		"url":      "/serve/" + filename,
		"filename": filename,
	})
	if err != nil {
		c.Errorf("%v", err)
	}
}

func saveBlobFile(c appengine.Context, blobInfo *blobstore.BlobInfo) error {
	filename := blobInfo.Filename
	blobFile := &BlobFile{
		BlobKey:  blobInfo.BlobKey,
		Filename: filename,
	}
	key := blobFileKeyFromFilename(c, filename)
	_, err := datastore.Put(c, key, blobFile)
	return err
}

func blobFileKeyFromFilename(c appengine.Context, filename string) *datastore.Key {
	return datastore.NewKey(
		c,          // appengine.Context
		"BlobFile", // Kind
		filename,   // String ID; empty means no string ID
		0,          // Integer ID; if 0, generate automatically. Ignored if string ID specified.
		nil,        // Parent Key; nil means no parent
	)
}

func init() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/serve/", handleServe)
	http.HandleFunc("/upload", handleUpload)
}
