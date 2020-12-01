package ini4go

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const (
	kDefaultSection = "default"
)

var sectionRegexp = regexp.MustCompile(`^\[(?P<header>[^]]+)\]$`)

func getSectionName(src string) (name string) {
	var rList = sectionRegexp.FindStringSubmatch(src)
	if len(rList) >= 2 {
		name = rList[1]
	}
	return name
}

var optionRegexp = regexp.MustCompile(`(?P<key>[^:=\s][^:=]*)\s*(?:(?P<vi>[:=])\s*(?P<value>.*))?$`)

func getOptionAndValue(src string) (option, vi, value string) {
	var rList = optionRegexp.FindStringSubmatch(src)
	if len(rList) >= 4 {
		option, vi, value = rList[1], rList[2], rList[3]
	}
	return option, vi, value
}

type Ini struct {
	iniParser
}

//func New() *Config {
//	return NewConfigWithBlock(true, true)
//}

func New(block bool) *Ini {
	var c = &Ini{}
	c.block = block
	c.init()
	return c
}

type iniParser struct {
	mutex        sync.RWMutex
	sectionKeys  []string
	sections     map[string]*Section
	block        bool
	uniqueOption bool
}

func (this *iniParser) Lock() {
	if this.block {
		this.mutex.Lock()
	}
}

func (this *iniParser) Unlock() {
	if this.block {
		this.mutex.Unlock()
	}
}

func (this *iniParser) RLock() {
	if this.block {
		this.mutex.RLock()
	}
}

func (this *iniParser) RUnlock() {
	if this.block {
		this.mutex.RUnlock()
	}
}

func (this *iniParser) SetUniqueOption(unique bool) {
	this.uniqueOption = unique
}

func (this *iniParser) init() {
	this.sectionKeys = nil
	this.sections = make(map[string]*Section)
}

func (this *iniParser) Load(dir string) error {
	var fileInfo, err = os.Stat(dir)
	if err != nil {
		return err
	}

	var pathList []string

	if fileInfo.IsDir() {
		var file *os.File
		file, err = os.Open(dir)
		if err != nil {
			return err
		}

		var names []string
		names, err = file.Readdirnames(-1)

		file.Close()
		if err != nil {
			return err
		}

		for _, name := range names {
			var filePath = path.Join(dir, name)
			fileInfo, err = os.Stat(filePath)
			if err != nil {
				continue
			}

			if !fileInfo.IsDir() {
				pathList = append(pathList, filePath)
			}
		}
	} else {
		pathList = append(pathList, dir)
	}

	return this.LoadFiles(pathList...)
}

func (this *iniParser) LoadFiles(files ...string) error {
	this.Lock()
	defer this.Unlock()

	for _, file := range files {
		var ext = filepath.Ext(file)
		if ext != ".ini" && ext != ".conf" {
			continue
		}

		var f, err = os.OpenFile(file, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		err = this.load(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *iniParser) load(r io.Reader) error {
	var reader = bufio.NewReader(r)
	var line []byte
	var err error

	var currentSection *Section

	var index = 0
	var comments []string
	for {
		if line, _, err = reader.ReadLine(); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if index == 0 {
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}
		index++

		var sLine = strings.TrimSpace(string(line))

		// 如果是注释或者空行,则忽略
		if sLine == "" {
			continue
		}

		if strings.HasPrefix(sLine, "#") || strings.HasPrefix(sLine, ";") {
			comments = append(comments, strings.TrimSpace(sLine[1:]))
			continue
		}

		var sectionName = getSectionName(sLine)
		if len(sectionName) > 0 && sectionName != kDefaultSection {
			currentSection = this.newSection(sectionName)
			currentSection.comments = append(currentSection.comments, comments...)
			comments = nil
			continue
		}

		if currentSection == nil {
			currentSection = this.newSection(kDefaultSection)
		}

		var optName, optIV, optValue = getOptionAndValue(sLine)
		optName = strings.TrimSpace(optName)
		optIV = strings.TrimSpace(optIV)
		optValue = strings.TrimSpace(optValue)

		if optName != "" {
			if this.uniqueOption && currentSection.HasOption(optName) {
				return errors.New("有重复的 Option: " + optName)
			}

			var opt = currentSection.newOption(optName, optIV)
			opt.AddValue(optValue)
			opt.AddComment(comments...)
			comments = nil
		}
	}
	return nil
}

func (this *iniParser) WriteToFile(file string) error {
	var f, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_SYNC, os.ModePerm)
	if err != nil {
		return err
	}

	f.Truncate(0)

	err = this.writeTo(f)
	f.Close()
	return err
}

func (this *iniParser) writeTo(w io.Writer) error {
	this.Lock()
	defer this.Unlock()

	var writer = bufio.NewWriter(w)

	for index, sectionName := range this.sectionKeys {

		if index > 0 {
			writer.WriteByte('\n')
		}

		var section = this.section(sectionName)
		for _, c := range section.Comments() {
			if len(strings.TrimSpace(c)) > 0 {
				writer.WriteString(fmt.Sprintf("# %s\n", c))
			}
		}

		writer.WriteString(fmt.Sprintf("[%s]\n", sectionName))

		for _, optionKey := range section.optionKeys {
			var opt = section.Option(optionKey)
			if len(opt.Comments()) > 0 {
				writer.WriteString("\n")
			}
			for _, c := range opt.Comments() {
				if len(strings.TrimSpace(c)) > 0 {
					writer.WriteString(fmt.Sprintf("# %s\n", c))
				}
			}
			for _, value := range opt.values {
				writer.WriteString(fmt.Sprintf("%s %s %s\n", opt.key, opt.iv, value))
			}
		}
	}
	return writer.Flush()
}

func (this *iniParser) Reset() {
	this.Lock()
	this.Unlock()
	this.init()
}

func (this *iniParser) newSection(name string) *Section {
	var section = this.sections[name]
	if section == nil {
		section = NewSection(name)
		this.sections[name] = section
		this.sectionKeys = append(this.sectionKeys, name)
	}
	return section
}

func (this *iniParser) NewSection(name string) *Section {
	this.Lock()
	defer this.Unlock()

	return this.newSection(name)
}

func (this *iniParser) mustSection(name string) *Section {
	return this.newSection(name)
}

func (this *iniParser) MustSection(name string) *Section {
	return this.NewSection(name)
}

func (this *iniParser) section(name string) *Section {
	var s, _ = this.sections[name]
	return s
}

func (this *iniParser) Section(name string) *Section {
	this.RLock()
	defer this.RUnlock()
	return this.section(name)
}

func (this *iniParser) SectionNames() []string {
	this.RLock()
	defer this.RUnlock()

	var names = make([]string, len(this.sectionKeys))
	copy(names, this.sectionKeys)
	return names
}

func (this *iniParser) SectionList() []*Section {
	this.RLock()
	defer this.RUnlock()

	var sList = make([]*Section, 0, len(this.sections))
	for _, value := range this.sections {
		sList = append(sList, value)
	}
	return sList
}

func (this *iniParser) HasSection(section string) bool {
	this.RLock()
	defer this.RUnlock()

	var _, ok = this.sections[section]
	return ok
}

func (this *iniParser) RemoveSection(section string) {
	this.Lock()
	defer this.Unlock()

	//if strings.ToLower(section) == kDefaultSection {
	//	return
	//}
	if section == kDefaultSection {
		return
	}
	delete(this.sections, section)

	var index = -1
	for i, key := range this.sectionKeys {
		if key == section {
			index = i
			break
		}
	}

	if index >= 0 {
		this.sectionKeys = append(this.sectionKeys[0:index], this.sectionKeys[index+1:]...)
	}
}

func (this *iniParser) mustOption(section, option string) *Option {
	var s = this.mustSection(section)
	var opt = s.MustOption(option)
	return opt
}

func (this *iniParser) MustOption(section, option string) *Option {
	this.Lock()
	defer this.Unlock()

	return this.mustOption(section, option)
}

func (this *iniParser) option(section, option string) *Option {
	var s = this.section(section)
	if s != nil {
		return s.Option(option)
	}
	return nil
}

func (this *iniParser) Option(section, option string) *Option {
	this.Lock()
	defer this.Unlock()
	return this.option(section, option)
}

func (this *iniParser) Options(section string) []string {
	this.RLock()
	defer this.RUnlock()

	var s = this.section(section)
	if s != nil {
		return s.OptionKeys()
	}
	return nil
}

func (this *iniParser) OptionList(section string) []*Option {
	this.RLock()
	defer this.RUnlock()

	var s = this.section(section)
	if s != nil {
		return s.OptionList()
	}
	return nil
}

func (this *iniParser) HasOption(section, option string) bool {
	this.RLock()
	defer this.RUnlock()

	if s, ok := this.sections[section]; ok {
		if _, ok := s.options[option]; ok {
			return true
		}
	}
	return false
}

func (this *iniParser) RemoveOption(section, option string) {
	this.Lock()
	defer this.Unlock()

	var s = this.sections[section]
	if s != nil {
		s.RemoveOption(option)
	}
}

func (this *iniParser) SetValue(section, option, value string) {
	this.Lock()
	defer this.Unlock()

	var s = this.newSection(section)
	var opt = s.newOption(option, "=")
	opt.SetValue(value)
}

func (this *iniParser) SetString(section, option, value string) {
	this.SetValue(section, option, value)
}

func (this *iniParser) SetInt(section, option string, value int) {
	this.SetValue(section, option, fmt.Sprintf("%d", value))
}

func (this *iniParser) SetInt64(section, option string, value int64) {
	this.SetValue(section, option, fmt.Sprintf("%d", value))
}

func (this *iniParser) SetFloat32(section, option string, value float32) {
	this.SetValue(section, option, fmt.Sprintf("%f", value))
}

func (this *iniParser) SetFloat64(section, option string, value float64) {
	this.SetValue(section, option, fmt.Sprintf("%f", value))
}

func (this *iniParser) SetBool(section, option string, value bool) {
	this.SetValue(section, option, fmt.Sprintf("%t", value))
}

func (this *iniParser) GetValue(section, option string) string {
	return this.MustValue(section, option, "")
}

func (this *iniParser) MustValue(section, option, defaultValue string) string {
	this.RLock()
	defer this.RUnlock()
	var opt = this.mustOption(section, option)
	return opt.MustString(defaultValue)
}

func (this *iniParser) MustInt(section, option string, defaultValue int) int {
	this.RLock()
	defer this.RUnlock()

	var opt = this.mustOption(section, option)
	return opt.MustInt(defaultValue)
}

func (this *iniParser) MustInt64(section, option string, defaultValue int64) int64 {
	this.RLock()
	defer this.RUnlock()

	var opt = this.mustOption(section, option)
	return opt.MustInt64(defaultValue)
}

func (this *iniParser) MustFloat32(section, option string, defaultValue float32) float32 {
	this.RLock()
	defer this.RUnlock()

	var opt = this.mustOption(section, option)
	return opt.MustFloat32(defaultValue)
}

func (this *iniParser) MustFloat64(section, option string, defaultValue float64) float64 {
	this.RLock()
	defer this.RUnlock()

	var opt = this.mustOption(section, option)
	return opt.MustFloat64(defaultValue)
}

func (this *iniParser) MustBool(section, option string, defaultValue bool) bool {
	this.RLock()
	defer this.RUnlock()

	var opt = this.mustOption(section, option)
	return opt.MustBool(defaultValue)
}

func (this *iniParser) GetValues(section, option string) []string {
	this.RLock()
	defer this.RUnlock()

	var s = this.sections[section]
	if s != nil {
		var opt = s.options[option]
		if opt != nil {
			return opt.Values()
		}
	}
	return nil
}
