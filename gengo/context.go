package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func isRosPackage(dir string) bool {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, f := range files {
		if f.Name() == "package.xml" {
			return true
		}
	}
	return false
}

func findAllMessages(rosPkgPaths []string) (map[string]string, error) {
	msgs := make(map[string]string)
	for _, p := range rosPkgPaths {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			pkgPath := filepath.Join(p, f.Name())
			if isRosPackage(pkgPath) {
				pkgName := filepath.Base(pkgPath)
				msgPath := filepath.Join(pkgPath, "msg")
				msgPaths, err := filepath.Glob(msgPath + "/*.msg")
				if err != nil {
					continue
				}
				for _, m := range msgPaths {
					basename := filepath.Base(m)
					rootName := basename[:len(basename)-4]
					fullName := pkgName + "/" + rootName
					msgs[fullName] = m
				}
			}
		}
	}
	return msgs, nil
}

func findAllServices(rosPkgPaths []string) (map[string]string, error) {
	srvs := make(map[string]string)
	for _, p := range rosPkgPaths {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			pkgPath := filepath.Join(p, f.Name())
			if isRosPackage(pkgPath) {
				pkgName := filepath.Base(pkgPath)
				srvPath := filepath.Join(pkgPath, "srv")
				srvPaths, err := filepath.Glob(srvPath + "/*.srv")
				if err != nil {
					continue
				}
				for _, m := range srvPaths {
					basename := filepath.Base(m)
					rootName := basename[:len(basename)-4]
					fullName := pkgName + "/" + rootName
					srvs[fullName] = m
				}
			}
		}
	}
	return srvs, nil
}

type MsgContext struct {
	msgPathMap  map[string]string
	srvPathMap  map[string]string
	msgRegistry map[string]*MsgSpec
}

func NewMsgContextFromEnv() (*MsgContext, error) {
	rosPkgPath := os.Getenv("ROS_PACKAGE_PATH")
	return NewMsgContext(strings.Split(rosPkgPath, ":"))
}

func NewMsgContext(rosPkgPaths []string) (*MsgContext, error) {
	ctx := new(MsgContext)
	msgs, err := findAllMessages(rosPkgPaths)
	if err != nil {
		return nil, err
	}
	ctx.msgPathMap = msgs

	srvs, err := findAllServices(rosPkgPaths)
	if err != nil {
		return nil, err
	}
	ctx.srvPathMap = srvs

	ctx.msgRegistry = make(map[string]*MsgSpec)
	return ctx, nil
}

func (ctx *MsgContext) Register(fullName string, spec *MsgSpec) {
	ctx.msgRegistry[fullName] = spec
}

func (ctx *MsgContext) LoadMsgFromString(text string, fullName string) (*MsgSpec, error) {
	packageName, shortName, e := packageResourceName(fullName)
	if e != nil {
		return nil, e
	}

	var fields []Field
	var constants []Constant
	for lineno, origLine := range strings.Split(text, "\n") {
		cleanLine := stripComment(origLine)
		if len(cleanLine) == 0 {
			// Skip empty line
			continue
		} else if strings.Contains(cleanLine, ConstChar) {
			constant, e := loadConstantLine(origLine)
			if e != nil {
				return nil, NewSyntaxError(fullName, lineno, e.Error())
			}
			constants = append(constants, *constant)
		} else {
			field, e := loadFieldLine(origLine, packageName)
			if e != nil {
				return nil, NewSyntaxError(fullName, lineno, e.Error())
			}
			fields = append(fields, *field)
		}
	}
	spec, _ := NewMsgSpec(fields, constants, text, fullName, OptionPackageName(packageName), OptionShortName(shortName))
	var err error
	md5sum, err := ctx.ComputeMsgMD5(spec)
	if err != nil {
		return nil, err
	}
	spec.MD5Sum = md5sum
	ctx.Register(fullName, spec)
	return spec, nil
}

func (ctx *MsgContext) LoadMsgFromFile(filePath string, fullName string) (*MsgSpec, error) {
	bs, e := ioutil.ReadFile(filePath)
	if e != nil {
		return nil, e
	}
	text := string(bs)
	return ctx.LoadMsgFromString(text, fullName)
}

func (ctx *MsgContext) LoadMsg(fullName string) (*MsgSpec, error) {
	if spec, ok := ctx.msgRegistry[fullName]; ok {
		return spec, nil
	} else {
		if path, ok := ctx.msgPathMap[fullName]; ok {
			spec, err := ctx.LoadMsgFromFile(path, fullName)
			if err != nil {
				return nil, err
			} else {
				ctx.msgRegistry[fullName] = spec
				return spec, nil
			}
		} else {
			return nil, fmt.Errorf("message definition of `%s` is not found", fullName)
		}
	}
}

func (ctx *MsgContext) LoadSrvFromString(text string, fullName string) (*SrvSpec, error) {
	packageName, shortName, err := packageResourceName(fullName)
	if err != nil {
		return nil, err
	}

	components := strings.Split(text, "---")
	if len(components) != 2 {
		return nil, fmt.Errorf("syntax error: missing '---'")
	}

	reqText := components[0]
	resText := components[1]

	reqSpec, err := ctx.LoadMsgFromString(reqText, fullName+"Request")
	if err != nil {
		return nil, err
	}
	resSpec, err := ctx.LoadMsgFromString(resText, fullName+"Response")
	if err != nil {
		return nil, err
	}

	spec := &SrvSpec{
		packageName, shortName, fullName, text, "", reqSpec, resSpec,
	}
	md5sum, err := ctx.ComputeSrvMD5(spec)
	if err != nil {
		return nil, err
	}
	spec.MD5Sum = md5sum

	return spec, nil
}

func (ctx *MsgContext) LoadSrvFromFile(filePath string, fullName string) (*SrvSpec, error) {
	bs, e := ioutil.ReadFile(filePath)
	if e != nil {
		return nil, e
	}
	text := string(bs)
	return ctx.LoadSrvFromString(text, fullName)
}

func (ctx *MsgContext) LoadSrv(fullName string) (*SrvSpec, error) {
	if path, ok := ctx.srvPathMap[fullName]; ok {
		spec, err := ctx.LoadSrvFromFile(path, fullName)
		if err != nil {
			return nil, err
		} else {
			return spec, nil
		}
	} else {
		return nil, fmt.Errorf("service definition of `%s` is not found", fullName)
	}
}

func (ctx *MsgContext) ComputeMD5Text(spec *MsgSpec) (string, error) {
	var buf bytes.Buffer
	for _, c := range spec.Constants {
		buf.WriteString(fmt.Sprintf("%s %s=%s\n", c.Type, c.Name, c.ValueText))
	}
	for _, f := range spec.Fields {
		if f.Package == "" {
			buf.WriteString(fmt.Sprintf("%s %s\n", f.Type, f.Name))
		} else {
			subspec, err := ctx.LoadMsg(f.Package + "/" + f.Type)
			if err != nil {
				return "", nil
			}
			submd5, err := ctx.ComputeMsgMD5(subspec)
			if err != nil {
				return "", nil
			}
			buf.WriteString(fmt.Sprintf("%s %s\n", submd5, f.Name))
		}
	}
	return strings.Trim(buf.String(), "\n"), nil
}

func (ctx *MsgContext) ComputeMsgMD5(spec *MsgSpec) (string, error) {
	md5text, err := ctx.ComputeMD5Text(spec)
	if err != nil {
		return "", err
	}
	hash := md5.New()
	hash.Write([]byte(md5text))
	sum := hash.Sum(nil)
	md5sum := hex.EncodeToString(sum)
	return md5sum, nil
}

func (ctx *MsgContext) ComputeSrvMD5(spec *SrvSpec) (string, error) {
	reqText, err := ctx.ComputeMD5Text(spec.Request)
	if err != nil {
		return "", err
	}
	resText, err := ctx.ComputeMD5Text(spec.Response)
	if err != nil {
		return "", err
	}
	hash := md5.New()
	hash.Write([]byte(reqText))
	hash.Write([]byte(resText))
	sum := hash.Sum(nil)
	md5sum := hex.EncodeToString(sum)
	return md5sum, nil
}
