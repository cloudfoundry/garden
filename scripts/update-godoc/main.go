package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	cmd := exec.Command("go", "list", "./...")
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Fatal(err)
	}

	packages := strings.Split(string(output), "\n")
	wg := new(sync.WaitGroup)
	for _, pkg := range packages {
		if pkg == "" {
			continue
		}
		wg.Add(1)
		go updatePackageGodoc(pkg, wg)
	}
	wg.Wait()
}

func updatePackageGodoc(pkg string, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Updating godoc for %s\n", pkg)
	body := fmt.Sprintf("path=%s", url.QueryEscape(pkg))
	resp, err := http.Post("http://godoc.org/-/refresh", "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		log.Printf("Failed to update godoc for package %s. error was %+v\n", pkg, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Response for updating package %s was expected to be 200, was %d", pkg, resp.StatusCode)
	}
}
