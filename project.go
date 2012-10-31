package seven5

import (
	"os"
	"path/filepath"
	"net/http"
	"fmt"
	"strings"
	"log"
	"bytes"
)

//ProjDirectoryFromGOPATH computes a directory inside the project level of a seven5 project
//that has the default layout.  For a project foo
// foo/
//    dart/
//    db/
//    go/
//         bin/
//         pkg/
//         src/
//               foo/
//    web/
// 
func ProjDirectoryFromGOPATH(rootDir string) (string, error) {
	env := os.Getenv("GOPATH")
	if env == "" {
		return "", BAD_GOPATH
	}
	pieces := strings.Split(env, ":")
	if len(pieces) > 1 {
		env = pieces[0]
	}
	return filepath.Join(filepath.Dir(env), rootDir), nil
}

// ProjRootDirFromPath returns the directory of the project's go source dir root.
func ProjRootDirFromPath(projectName string) (string, error) {
	return ProjDirectoryFromGOPATH(filepath.Join("go","src", projectName))
}

//StaticContent adds an http handler for static content in a subdirectory
func StaticContent(h Handler, urlPath string, subdir string) {
	//setup static content
	truePath, err := ProjDirectoryFromGOPATH(subdir)
	if err != nil {
		log.Fatalf("can't understand GOPATH or not using default project layout: %s", err)
	}
	//strip the path from requests so that /urlPath/fart = modena/subdir/fart
	h.ServeMux().Handle(urlPath, http.StripPrefix(urlPath, http.FileServer(http.Dir(truePath))))
}

//GeneratedContent adds an http handler for static content in a subdirectory.  It computes
//the generated code at the point of call, so it should be called after resources are already
//bound.
func GeneratedContent(h Handler, urlPath string) {
	desc := h.Resources()
	h.ServeMux().HandleFunc(fmt.Sprintf("%sdart", urlPath), generateDartFunc(desc))
}

//Seven5Content maps /seven5/ to be the place where static content contained _inside_
//the seven5 can viewed such as /seven5/seven5.dart for the seven5 support library.
func Seven5Content(h Handler, urlPath string, projectName string) {
	h.ServeMux().HandleFunc(fmt.Sprintf("%sseven5.dart", urlPath),
		generateStringPrinter(seven5_dart, "text/plain"))
	if projectName!="" {
		h.ServeMux().HandleFunc(fmt.Sprintf("%spublicsetting/%s/", urlPath, projectName), publicSettingHandler)
	}
}

//SetIcon creates a go handler in h that will return an icon to be displayed in response to /favicon.ico.
//The binaryIcon should be an array of bytes (usually created via 'seven5tool embedfile')
func SetIcon(h Handler, binaryIcon []byte) {
	h.ServeMux().HandleFunc("/favicon.ico", generateBinPrinter(binaryIcon, "image/x-icon"))
}
func generateStringPrinter(content string, contentType string) func(http.ResponseWriter, *http.Request) {
	return generateBinPrinter([]byte(content), contentType)
}

func generateBinPrinter(content []byte, contentType string) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Add("Content-type", contentType)
		_, err := writer.Write(content)
		if err != nil {
			fmt.Printf("error writing constant binary string: %s\n", err)
		}
	}
}

//generateDartFunc returns a function that outputs text string for all the dart code
//in the system.
func generateDartFunc(desc []*ResourceDescription) func(http.ResponseWriter, *http.Request) {

	var text bytes.Buffer
	resourceStructs := []*FieldDescription{}
	supportStructs := []*FieldDescription{}

	//generate code for known resources
	for _, d := range desc {
		text.WriteString(generateDartForResource(d))
		resourceStructs = append(resourceStructs, d.Field)
	}
	//collect up supporting structs
	for _, d := range desc {
		candidates := collectStructs(d.Field)
		for _, s := range candidates {
			if !containsType(resourceStructs, s) && !containsType(supportStructs, s) {
				supportStructs = append(supportStructs, s)
			}
		}
	}
	for _, i := range supportStructs {
		text.WriteString(generateDartForSupportStruct(i))
	}
	return generateBinPrinter(text.Bytes(), "text/plain")
}

//DefaultProjects adds the resources that we expect to be present for a typical
//seven5 project to the handler provided and returns it as as http.Handler so it can be
//use "in the normal way" with http.ServeHttp. This adds generated code to the URL
//mount point /generated.  If you call DefaultProjectBindings you don't need to worry
//about calling StaticContent or GeneratedContent.  Note that this should be called
//_after_ all resources are added to the handler as there is caching of the generated
//code.  If the project name is "" then the publicsetting.json file will not be read and
//no settings will be visible from the client side (nothing at /seven5/publicsetting)
func DefaultProjectBindings(h Handler, projectName string) http.Handler {
	StaticContent(h, "/static/", "static")
	StaticContent(h, "/dart/", "dart")
	GeneratedContent(h, "/generated/")
	Seven5Content(h, "/seven5/", projectName)
	SetIcon(h, gopher_ico)
	return h
}

const (
	WAITING_ON_NON_WS = iota
	WAITING_ON_EOL
)

//dartPrettyPrint is a very naive dart formatter. It doesn't understand much of the lexical
//structure of dart but it's enough for our generated code (which doesn't do things like embed
//{ inside a string and does has too many, not too few, line breaks)
func dartPrettyPrint(raw string) []byte {

	state := WAITING_ON_NON_WS
	indent := 0
	var result bytes.Buffer

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch state {
		case WAITING_ON_NON_WS:
			if c == '\t' || c == ' ' || c == '\n' {
				continue
			}
			switch c {
			case '{':
				indent += 2
			case '}':
				indent -= 2
			}
			for j := 0; j < indent; j++ {
				result.WriteString(" ")
			}
			result.Write([]byte{c})
			state = WAITING_ON_EOL
			continue
		case WAITING_ON_EOL:
			if c == '\n' {
				result.WriteString("\n")
				state = WAITING_ON_NON_WS
				continue
			}
			switch c {
			case '{':
				indent += 2
			case '}':
				indent -= 2
			}
			result.Write([]byte{c})
			continue
		}
	}
	return result.Bytes()
}