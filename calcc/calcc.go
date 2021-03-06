// Copyright (c) 2014, Rob Thornton
// All rights reserved.
// This source code is governed by a Simplied BSD-License. Please see the
// LICENSE included in this distribution for a copy of the full license
// or, if one is not included, you may also find a copy at
// http://opensource.org/licenses/BSD-2-Clause

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rthornton128/calc/comp"
)

func cleanup(filename string) {
	os.Remove(filename + ".c")
	os.Remove(filename + ".o")
}

func fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func findRuntime() string {
	var paths []string
	rpath := "/src/github.com/rthornton128/calc/runtime"
	if runtime.GOOS == "Windows" {
		paths = strings.Split(os.Getenv("GOPATH"), ";")
	} else {
		paths = strings.Split(os.Getenv("GOPATH"), ":")
	}
	for _, path := range paths {
		path = filepath.Join(path, rpath)
		_, err := os.Stat(filepath.Join(path, "runtime.a"))
		if err == nil || os.IsExist(err) {
			return path
		}
	}
	return ""
}

func make_args(options ...string) string {
	var args string
	for i, opt := range options {
		if len(opt) > 0 {
			args += opt
			if i < len(options)-1 {
				args += " "
			}
		}
	}
	return args
}

func printVersion() {
	fmt.Fprintln(os.Stderr, "Calc Compiler Tool Version 2.0")
}

func main() {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	flag.Usage = func() {
		printVersion()
		fmt.Fprintln(os.Stderr, "\nUsage of:", os.Args[0])
		fmt.Fprintln(os.Stderr, os.Args[0], "[flags] <filename>")
		flag.PrintDefaults()
	}
	var (
		asm  = flag.Bool("s", false, "generate C code but do not compile")
		cc   = flag.String("cc", "gcc", "C compiler to use")
		cfl  = flag.String("cflags", "-c -std=gnu99", "C compiler flags")
		cout = flag.String("cout", "--output=", "C compiler output flag")
		ld   = flag.String("ld", "gcc", "linker")
		ldf  = flag.String("ldflags", "", "linker flags")
		ver  = flag.Bool("v", false, "Print version number and exit")
	)
	flag.Parse()

	if *ver {
		printVersion()
		os.Exit(1)
	}
	var filename, path string
	switch flag.NArg() {
	case 0:
		path, _ = filepath.Abs(".")
	case 1:
		path, _ = filepath.Abs(flag.Arg(0))
	default:
		flag.Usage()
		os.Exit(1)
	}
	path, filename = filepath.Split(path)

	/* do a preemptive search to see if runtime can be found. Does not
	 * guarantee it will be there at link time */
	rpath := findRuntime()
	if rpath == "" {
		fatal("Unable to find runtime in GOPATH. Make sure 'make' command was " +
			"run in source directory")
	}

	fmt.Println("Compiling:", filename)

	fi, err := os.Stat(filepath.Join(path, filename))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if fi.IsDir() {
		//fmt.Println("calling compile dir")
		comp.CompileDir(filepath.Join(path, filename))
	} else {
		//fmt.Println("calling compile file")
		comp.CompileFile(filepath.Join(path, filename))
	}
	path = filepath.Join(path, filename)
	path = path[:len(path)-len(filepath.Ext(path))]

	if !*asm {
		/* compile to object code */
		var out []byte
		args := make_args(*cfl, "-I "+rpath, *cout+path+".o", path+".c")
		out, err := exec.Command(*cc+ext, strings.Split(args, " ")...).CombinedOutput()
		if err != nil {
			cleanup(path)
			fatal(string(out), err)
		}

		/* link to executable */
		args = make_args(*ldf, *cout+path+ext, path+".o", rpath+"/runtime.a")
		out, err = exec.Command(*ld+ext, strings.Split(args, " ")...).CombinedOutput()
		if err != nil {
			cleanup(path)
			fatal(string(out), err)
		}
		cleanup(path)
	}
}
