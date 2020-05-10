package app

import (
	"fmt"

	"github.com/kuetemeier/imgindex/imgmeta"
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
func GetBasicInfo(img imgmeta.Image) (info BasicInfo) {
	width, err := img.ReadTagValue("SOF0", imgmeta.SOF0ImageWidth)
	if err == nil {
		info.Width = width
	} else {
		fmt.Println(err.Error())
	}
	height, err := img.ReadTagValue("SOF0", imgmeta.SOF0ImageHeight)
	if err == nil {
		info.Height = height.(uint32)
	} else {
		fmt.Println(err.Error())
	}
	keyword, err := img.ReadTagValue("IPTC", imgmeta.IptcTagApplication2Keywords)
	if err == nil {
		info.Keywords = []string{keyword.(string)}
	}
	datetime, err := img.ReadTagValue("EXIF", imgmeta.ExifTagDateTimeOriginal)
	if err == nil {
		fmt.Printf("datetime:%v\n", datetime)
	}
	//height, err := img.ReadTagValue("IPTC", IptcTagApplication2Keywords)
	//if err == nil {
	//	info.Height = height.(float64)
	//} else {
	//	fmt.Println(err.Error())
	//}

	imgTitle, err := img.ReadTagValue("IPTC", imgmeta.IptcTagApplication2Caption)
	if err == nil {
		info.Title = imgTitle.(string)
	}

	return
}
