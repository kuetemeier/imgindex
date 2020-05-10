/*
Copyright © 2020 Jörg Kütemeier <joerg@kuetemeier.de>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package app holds all app related work.
package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kuetemeier/imgindex/imgmeta"
)

// Index start the index process
func Index() {

	fhnd, err := os.Open("testdata/the-wall-sample.jpg")
	if err != nil {
		return
	}

	image, err := imgmeta.ReadJpeg(fhnd)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	basicInfo := GetBasicInfo(image)
	fmt.Printf("Title: %v", basicInfo.Title)
	fmt.Printf("Image: width:%v, height:%v\n", basicInfo.Width, basicInfo.Height)
	fmt.Printf("Keywords: %v\n", basicInfo.Keywords)

}

func processSourceDir() {

	err := filepath.Walk("sample",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path, info.Size())
			return nil
		})
	if err != nil {
		log.Println(err)
	}

}
