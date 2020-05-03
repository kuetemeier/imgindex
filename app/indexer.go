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

	"github.com/rwcarlsen/goexif/exif"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

// Index start the index process
func Index() {

	var err error
	var imgFile *os.File
	var metaData *exif.Exif
	var jsonByte []byte
	var jsonString string

	if viper.GetBool("verbose") {
		fmt.Println("INFO: Starting index.")
	}

	imgFile, err = os.Open("sample/the-wall-sample.jpg")
	if err != nil {
		log.Fatal(err.Error())
	}

	metaData, err = exif.Decode(imgFile)
	if err != nil {
		log.Fatal(err.Error())
	}

	jsonByte, err = metaData.MarshalJSON()
	if err != nil {
		log.Fatal(err.Error())
	}

	jsonString = string(jsonByte)
	fmt.Println(jsonString)

	fmt.Println("Make: " + gjson.Get(jsonString, "Make").String())
	fmt.Println("Model: " + gjson.Get(jsonString, "Model").String())
	fmt.Println("Software: " + gjson.Get(jsonString, "Software").String())

}
