package seven5

import (
	"bytes"
	"fmt"
	"net/http"
	"seven5/util"
)

//simulate const array
func DEFAULT_IMPORTS() []string {
	return []string{"fmt", "seven5/plugin", "os"}
}

// Bootstrap is responsible for buliding the current seven5 executable
// based on the groupie configuration.
type bootstrap struct {
	request *http.Request
	logger  util.SimpleLogger
}

//Bootstrap is invoked from the roadie to tell us that the user wants to 
//try to build and run their project.  Normally, this results in a new
//Seven5 excutabel.
func Bootstrap(writer  http.ResponseWriter,request *http.Request) {
	logger:= util.NewHtmlLogger(util.DEBUG, true, writer)
	b:=&bootstrap{request, logger}	
	config:=b.configureSeven5()
	if config!=nil {
		b.takeSeven5Pill(config)
	}
}

//configureSeven5 checks for a goroupie config file and returns a config or
//nil in the error case.
func (self *bootstrap) configureSeven5() groupieConfig {

	var groupieJson string
	var err error
	var result groupieConfig

	self.logger.Debug("checking for groupies config file...")
	groupieJson, err = findGroupieConfigFile()
	if err != nil {
		self.logger.Error("unable find or open the groupies config:%s", err)
		return nil
	}
	self.logger.Debug("Groupies configuration:")
	self.logger.DumpJson(groupieJson)
	if result, err = getGroupies(groupieJson, self.logger); err != nil {
		return nil
	}

	return result
}

// getGroupies is called to read a set of groupie values
// from json to a config structures. It returns nil if the format is not 
// satisfactory (plus an error value).  Note that this does not check semantics!
func getGroupies(jsonBlob string, logger util.SimpleLogger) (groupieConfig, error) {
	var result groupieConfig
	var err error
	if result, err = parseGroupieConfig(jsonBlob); err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return result, nil
}


//takeSeven5 generates the pill in a temp directory and compiles it.  It returns
//the name of the new seven5 command or "" if it failed.
func (self *bootstrap) takeSeven5Pill(config groupieConfig) string {
	var cmd string
	var errText string
	var imports bytes.Buffer
	
	seen := util.NewBetterList()
	for _, i := range DEFAULT_IMPORTS() {
		seen.PushBack(i)
	}
	//gather all includes
	for _, v := range config {
		for _, i := range v.ImportsNeeded {
			if seen.Contains(i) {
				continue
			}
			seen.PushBack(i)
		}
	}
	for e := seen.Front(); e != nil; e = e.Next() {
		imports.WriteString(fmt.Sprintf("import \"%s\"\n", e.Value))
	}

	mainCode := fmt.Sprintf(seven5pill,
		imports.String(),
		config["ProjectValidator"].TypeName)

	if cmd, errText = util.CompilePill(mainCode, self.logger); cmd == "" {
		self.logger.DumpTerminal(mainCode)
		self.logger.DumpTerminal(errText)
		self.logger.Error("Unable to compile the seven5pill! Your plugins must be bogus!")
		return ""
	}
	self.logger.Info("Seven5 is now %s", cmd)
	return cmd
}

//seven5pill is the text of the pill
const seven5pill = `
package main
%s

func main() {
	plugin.Seven5App.SetProjectValidator(&%s{})
	if len(os.Args)<3 {
		os.Exit(1)
	}
	fmt.Println(plugin.Run(os.Args[1], os.Args[2]))
}
`
