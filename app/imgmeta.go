package app

import (
	"fmt"

	meta "github.com/kuetemeier/imgmeta/app/meta"
)

// BasicInfo contains the most basic information that could be asked for
type BasicInfo struct {
	Width    interface{}
	Height   uint32
	Title    string
	Descr    string
	Keywords []string
}

// GetBasicInfo gets the basic information from the meta-information of the image
func GetBasicInfo(img meta.Image) (info BasicInfo) {
	width, err := img.ReadTagValue("SOF0", meta.SOF0ImageWidth)
	if err == nil {
		info.Width = width
	} else {
		fmt.Println(err.Error())
	}
	height, err := img.ReadTagValue("SOF0", meta.SOF0ImageHeight)
	if err == nil {
		info.Height = height.(uint32)
	} else {
		fmt.Println(err.Error())
	}
	keyword, err := img.ReadTagValue("IPTC", meta.IptcTagApplication2Keywords)
	if err == nil {
		info.Keywords = []string{keyword.(string)}
	}
	datetime, err := img.ReadTagValue("EXIF", meta.ExifTagDateTimeOriginal)
	if err == nil {
		fmt.Printf("datetime:%v\n", datetime)
	}
	//height, err := img.ReadTagValue("IPTC", IptcTagApplication2Keywords)
	//if err == nil {
	//	info.Height = height.(float64)
	//} else {
	//	fmt.Println(err.Error())
	//}

	imgTitle, err := img.ReadTagValue("IPTC", meta.IptcTagApplication2Caption)
	if err == nil {
		info.Title = imgTitle.(string)
	}

	return
}
