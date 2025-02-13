package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func writeCode(fullName string, code string) error {
	nameComponents := strings.Split(fullName, "/")
	pkgDir := filepath.Join("vendor", nameComponents[0])
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		err = os.MkdirAll(pkgDir, os.ModeDir|os.FileMode(0775))
		if err != nil {
			return err
		}
	}
	filename := filepath.Join(pkgDir, nameComponents[1]+".go")

	return ioutil.WriteFile(filename, []byte(code), os.FileMode(0664))
}

func main() {
	if _, err := os.Stat("vendor"); os.IsNotExist(err) {
		err = os.Mkdir("vendor", os.ModeDir|os.FileMode(0775))
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}

	if len(os.Args) < 3 {
		fmt.Println("USAGE: gengo msg|srv <NAME> [<FILE>]")
		os.Exit(-1)
	}

	context, err := NewMsgContextFromEnv()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	mode := os.Args[1]
	fullName := os.Args[2]

	fmt.Printf("Generating %v...", fullName)

	if mode == "msg" {
		var spec *MsgSpec
		var err error
		if len(os.Args) == 3 {
			spec, err = context.LoadMsg(fullName)
		} else {
			spec, err = context.LoadMsgFromFile(os.Args[3], fullName)
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		var code string
		code, err = GenerateMessage(context, spec)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		err = writeCode(fullName, code)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	} else if mode == "srv" {
		var spec *SrvSpec
		var err error
		if len(os.Args) == 3 {
			spec, err = context.LoadSrv(fullName)
		} else {
			spec, err = context.LoadSrvFromFile(os.Args[3], fullName)
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		srvCode, reqCode, resCode, err := GenerateService(context, spec)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		err = writeCode(fullName, srvCode)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		err = writeCode(spec.Request.FullName, reqCode)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		err = writeCode(spec.Response.FullName, resCode)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	} else {
		fmt.Println("USAGE: genmsg <MSG>")
		os.Exit(-1)
	}
	fmt.Println("Done")
}
