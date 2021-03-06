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
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

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
		log.Error(err.Error())
		return
	}

	basicInfo := GetBasicInfo(image)
	log.Info(fmt.Sprintf("Title: %v", basicInfo.Title))
	log.Info(fmt.Sprintf("Image: width: %v, height: %v", basicInfo.Width, basicInfo.Height))
	log.Info(fmt.Sprintf("Keywords: %v", basicInfo.Keywords))

}

func processSourceDir() {

	err := filepath.Walk("testdata",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			log.Println(path, info.Size())
			return nil
		})
	if err != nil {
		log.Println(err)
	}

}
